package rtsp

import (
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"tuya-ipc-terminal/pkg/core"
	"tuya-ipc-terminal/pkg/utils"

	"github.com/pion/rtp"
)

// RTP transport mode (UDP or TCP)
type TransportMode int

const (
	TransportUDP TransportMode = iota
	TransportTCP               // Interleaved
)

type RTPForwarder struct {
	clients map[string]*RTPClient
	mutex   sync.RWMutex

	// RTP session info — accessed atomically because WebRTC callbacks,
	// RTSP forwarding and Stop() can run concurrently.
	videoSSRC atomic.Uint32
	audioSSRC atomic.Uint32

	// First-packet diagnostics — accessed atomically.
	videoFirstLogged atomic.Bool
	audioFirstLogged atomic.Bool

	// HEVC startup state. Parameter sets are cached from the camera stream so
	// a newly attached RTSP client can start on a clean random-access point.
	hevcEnabled    atomic.Bool
	hevcVPS        []byte
	hevcSPS        []byte
	hevcPPS        []byte
	hevcFUType     byte
	hevcFUBuf      []byte
	hevcFUActive   bool
	hevcFUSeq      uint16
	hevcFUSeqValid bool

	OnBackchannelAudio func(*rtp.Packet)
}

type RTPClient struct {
	sessionID     string
	transportMode TransportMode

	// UDP transport - Outgoing connections (server -> client)
	videoConn *net.UDPConn // For sending video to client
	audioConn *net.UDPConn // For sending audio to client

	// UDP transport - Client addresses
	videoAddr *net.UDPAddr
	audioAddr *net.UDPAddr

	// UDP transport - Client ports
	videoRTPPort          int // Client's video receiving port
	audioRTPPort          int // Client's audio receiving port
	backchannelClientPort int // Client's backchannel sending port

	// UDP backchannel listeners (server side)
	backchannelListener     *net.UDPConn // Server's RTP listener for backchannel
	backchannelRTCPListener *net.UDPConn // Server's RTCP listener for backchannel
	backchannelServerPort   int          // Server's RTP listening port
	backchannelRTCPPort     int          // Server's RTCP listening port

	// TCP interleaved transport
	tcpConn             net.Conn
	videoRTPChannel     byte
	audioRTPChannel     byte
	backAudioRTPChannel byte

	lastActivity atomic.Int64

	// Per-client HEVC startup/sequence state. Protected by RTPForwarder.mutex.
	hevcStarted  bool
	videoSeqInit bool
	nextVideoSeq uint16
	videoSSRC uint32

	// Per-client RTP sequence/SSRC state.
	audioSeqInit bool
	nextAudioSeq uint16
	audioSSRC    uint32
}

func newRTPSSRC() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		ssrc := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		if ssrc != 0 {
			return ssrc
		}
	}
	return uint32(time.Now().UnixNano())
}

func newRTPSequence() uint16 {
	var b [2]byte
	if _, err := rand.Read(b[:]); err == nil {
		return uint16(b[0])<<8 | uint16(b[1])
	}

	return uint16(time.Now().UnixNano())
}

func NewRTPForwarder() *RTPForwarder {
	rf := &RTPForwarder{
		clients: make(map[string]*RTPClient),
	}
	rf.audioSSRC.Store(1)
	return rf
}

func (rf *RTPForwarder) SetHEVC(enabled bool) {
	rf.hevcEnabled.Store(enabled)
}

func (rf *RTPForwarder) AddUDPClient(sessionID string, videoRTPPort, audioRTPPort int, remoteHost string) error {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	// Check if client already exists
	if client, exists := rf.clients[sessionID]; exists {
		// Update existing client with new ports
		client.videoRTPPort = videoRTPPort
		client.audioRTPPort = audioRTPPort
		client.lastActivity.Store(time.Now().UnixNano())
		client.hevcStarted = false
		client.videoSeqInit = false

		// Create new connections if needed
		if videoRTPPort > 0 && client.videoConn == nil {
			videoAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(remoteHost, strconv.Itoa(videoRTPPort)))
			videoConn, _ := net.DialUDP("udp", nil, videoAddr)
			client.videoAddr = videoAddr
			client.videoConn = videoConn
		}

		if audioRTPPort > 0 && client.audioConn == nil {
			audioAddr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort(remoteHost, strconv.Itoa(audioRTPPort)))
			audioConn, _ := net.DialUDP("udp", nil, audioAddr)
			client.audioAddr = audioAddr
			client.audioConn = audioConn
		}

		return nil
	}

	client := &RTPClient{
		sessionID:     sessionID,
		transportMode: TransportUDP,
		videoSSRC:     newRTPSSRC(),
		audioSSRC:     newRTPSSRC(),
		videoRTPPort:  videoRTPPort,
		audioRTPPort:  audioRTPPort,
	}

	// Create video connection if port provided
	if videoRTPPort > 0 {
		videoAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(remoteHost, strconv.Itoa(videoRTPPort)))
		if err != nil {
			return fmt.Errorf("failed to resolve video UDP address: %v", err)
		}

		videoConn, err := net.DialUDP("udp", nil, videoAddr)
		if err != nil {
			return fmt.Errorf("failed to create video UDP connection: %v", err)
		}

		client.videoAddr = videoAddr
		client.videoConn = videoConn
	}

	// Create audio connection if port provided
	if audioRTPPort > 0 {
		audioAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(remoteHost, strconv.Itoa(audioRTPPort)))
		if err != nil {
			if client.videoConn != nil {
				client.videoConn.Close()
			}
			return fmt.Errorf("failed to resolve audio UDP address: %v", err)
		}

		audioConn, err := net.DialUDP("udp", nil, audioAddr)
		if err != nil {
			if client.videoConn != nil {
				client.videoConn.Close()
			}
			return fmt.Errorf("failed to create audio UDP connection: %v", err)
		}

		client.audioAddr = audioAddr
		client.audioConn = audioConn
	}

	rf.clients[sessionID] = client

	core.Logger.Trace().Msgf("Added UDP RTP client %s (video port:%d, audio port:%d)",
		sessionID, videoRTPPort, audioRTPPort)
	return nil
}

func (rf *RTPForwarder) SetupUDPBackchannel(sessionID string, clientPort int) (int, error) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	client, exists := rf.clients[sessionID]
	if !exists {
		return 0, fmt.Errorf("client %s not found", sessionID)
	}

	if client.transportMode != TransportUDP {
		return 0, fmt.Errorf("client %s is not using UDP transport", sessionID)
	}

	// Store client's backchannel port
	client.backchannelClientPort = clientPort

	// If listeners already exist, return existing port
	if client.backchannelListener != nil {
		return client.backchannelServerPort, nil
	}

	// Allocate consecutive ports for RTP/RTCP
	portPair, err := utils.DefaultPortAllocator.GetConsecutiveUDPPorts(nil, 10)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate UDP ports for backchannel: %v", err)
	}

	// Store listeners and ports
	client.backchannelListener = portPair.RTPListener
	client.backchannelRTCPListener = portPair.RTCPListener
	client.backchannelServerPort = portPair.RTPPort
	client.backchannelRTCPPort = portPair.RTCPPort

	// Start goroutines to handle incoming packets
	go rf.handleUDPBackchannelRTP(sessionID, client.backchannelListener)
	go rf.handleUDPBackchannelRTCP(client.backchannelRTCPListener)

	core.Logger.Trace().Msgf("Setup UDP backchannel for client %s (client ports:%d-%d, server ports:%d-%d)",
		sessionID, clientPort, clientPort+1, portPair.RTPPort, portPair.RTCPPort)

	return portPair.RTPPort, nil
}

func (rf *RTPForwarder) AddTCPClient(sessionID string, conn net.Conn, videoRTPChannel, audioRTPChannel, backAudioRTPChannel byte) error {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	// Check if client already exists, update it
	if existingClient, exists := rf.clients[sessionID]; exists {
		core.Logger.Trace().Msgf("TCP client %s already exists, updating channels (video:%d->%d, audio:%d->%d, back:%d->%d)",
			sessionID, existingClient.videoRTPChannel, videoRTPChannel, existingClient.audioRTPChannel, audioRTPChannel, existingClient.backAudioRTPChannel, backAudioRTPChannel)
		existingClient.videoRTPChannel = videoRTPChannel
		existingClient.audioRTPChannel = audioRTPChannel
		existingClient.backAudioRTPChannel = backAudioRTPChannel
		existingClient.lastActivity.Store(time.Now().UnixNano())
		existingClient.hevcStarted = false
		existingClient.videoSeqInit = false
		existingClient.audioSeqInit = false
		existingClient.videoSSRC = newRTPSSRC()
		existingClient.audioSSRC = newRTPSSRC()
		return nil
	}

	client := &RTPClient{
		sessionID:           sessionID,
		transportMode:       TransportTCP,
		tcpConn:             conn,
		videoRTPChannel:     videoRTPChannel,
		audioRTPChannel:     audioRTPChannel,
		backAudioRTPChannel: backAudioRTPChannel,
	}

	rf.clients[sessionID] = client

	core.Logger.Trace().Msgf("Added TCP RTP client %s (video channel:%d, audio channel:%d, back audio channel:%d)",
		sessionID, videoRTPChannel, audioRTPChannel, backAudioRTPChannel)
	return nil
}

func (rf *RTPForwarder) RemoveClient(sessionID string) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	if client, exists := rf.clients[sessionID]; exists {
		if client.transportMode == TransportUDP {
			if client.videoConn != nil {
				_ = client.videoConn.Close()
			}
			if client.audioConn != nil {
				_ = client.audioConn.Close()
			}
			if client.backchannelListener != nil {
				_ = client.backchannelListener.Close()
			}
			if client.backchannelRTCPListener != nil {
				_ = client.backchannelRTCPListener.Close()
			}
		} else if client.tcpConn != nil {
			_ = client.tcpConn.Close()
		}

		delete(rf.clients, sessionID)
		core.Logger.Trace().Msgf("Removed RTP client %s", sessionID)
	}
}

func (rf *RTPForwarder) ForwardVideoPacket(packet *rtp.Packet) {
	rf.mutex.Lock()

	isHEVC := rf.hevcEnabled.Load()
	isRandomAccess := false

	if isHEVC {
		// Cache parameter sets even when no RTSP client is attached yet.
		isRandomAccess = rf.updateHEVCCache(packet.SequenceNumber, packet.Payload)
	}

	if len(rf.clients) == 0 {
		rf.mutex.Unlock()
		return
	}

	var deadClients []string
	now := time.Now()

	for sessionID, client := range rf.clients {
		client.lastActivity.Store(now.UnixNano())

		if isHEVC && !client.hevcStarted {
			// Do not feed a new decoder from the middle of a GOP.
			// Wait until a random-access NAL and all parameter sets are known.
			if !isRandomAccess ||
				len(rf.hevcVPS) == 0 ||
				len(rf.hevcSPS) == 0 ||
				len(rf.hevcPPS) == 0 {
				continue
			}

			// A new HEVC RTP stream gets a new SSRC and a new sequence space.
			// Do NOT hard-code sequence number 1.
			client.videoSSRC = newRTPSSRC()
			client.nextVideoSeq = newRTPSequence()
			client.videoSeqInit = true

			for _, parameterSet := range [][]byte{
				rf.hevcVPS,
				rf.hevcSPS,
				rf.hevcPPS,
			} {
				ps := *packet
				ps.SequenceNumber = client.nextVideoSeq
				ps.SSRC = client.videoSSRC
				client.nextVideoSeq++
				ps.Marker = false
				ps.Payload = parameterSet

				data, err := ps.Marshal()
				if err != nil {
					core.Logger.Error().
						Err(err).
						Msg("Error marshaling cached HEVC parameter set")
					continue
				}

				if err := rf.writeVideoPacket(client, data); err != nil {
					if isDeadClientError(err) {
						deadClients = append(deadClients, sessionID)
					} else {
						core.Logger.Error().
							Err(err).
							Msgf(
								"Error forwarding cached HEVC parameter set to RTP client %s",
								sessionID,
							)
					}
					break
				}
			}

			if containsString(deadClients, sessionID) {
				continue
			}

			client.hevcStarted = true
		}

		out := *packet

		if client.videoSSRC == 0 {
			client.videoSSRC = newRTPSSRC()
		}

		out.SSRC = client.videoSSRC

		if isHEVC {
			if !client.videoSeqInit {
				client.nextVideoSeq = newRTPSequence()
				client.videoSeqInit = true
			}

			out.SequenceNumber = client.nextVideoSeq
			client.nextVideoSeq++
		}

		data, err := out.Marshal()
		if err != nil {
			core.Logger.Error().
				Err(err).
				Msg("Error marshaling video RTP packet")
			continue
		}

		writeErr := rf.writeVideoPacket(client, data)
		if writeErr != nil {
			if isDeadClientError(writeErr) {
				deadClients = append(deadClients, sessionID)
			} else {
				core.Logger.Error().
					Err(writeErr).
					Msgf(
						"Error forwarding video RTP packet to RTP client %s",
						sessionID,
					)
			}
			continue
		}

		if !rf.videoFirstLogged.Swap(true) {
			if client.transportMode == TransportUDP {
				core.Logger.Trace().Msgf(
					"Successfully sent first video packet to UDP client %s on port %d",
					sessionID,
					client.videoRTPPort,
				)
			} else {
				core.Logger.Trace().Msgf(
					"Successfully sent first video packet to TCP client %s on channel %d",
					sessionID,
					client.videoRTPChannel,
				)
			}
		}
	}

	for _, sessionID := range deadClients {
		if client, ok := rf.clients[sessionID]; ok {
			rf.closeClient(client)
			delete(rf.clients, sessionID)
			core.Logger.Debug().
				Msgf("Removing dead video client %s", sessionID)
		}
	}

	rf.mutex.Unlock()
}

func (rf *RTPForwarder) writeVideoPacket(client *RTPClient, data []byte) error {
	if client.transportMode == TransportUDP {
		if client.videoConn != nil {
			_, err := client.videoConn.Write(data)
			return err
		}
		return nil
	}
	if client.transportMode == TransportTCP && client.tcpConn != nil {
		return rf.sendInterleavedRTP(client.tcpConn, client.videoRTPChannel, data)
	}
	return nil
}

func (rf *RTPForwarder) closeClient(client *RTPClient) {
	if client.transportMode == TransportUDP {
		if client.videoConn != nil {
			_ = client.videoConn.Close()
		}
		if client.audioConn != nil {
			_ = client.audioConn.Close()
		}
		if client.backchannelListener != nil {
			_ = client.backchannelListener.Close()
		}
		if client.backchannelRTCPListener != nil {
			_ = client.backchannelRTCPListener.Close()
		}
	} else if client.tcpConn != nil {
		_ = client.tcpConn.Close()
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// updateHEVCCache caches complete VPS/SPS/PPS NAL units carried either as a
// single NAL unit or inside an aggregation packet. It also reports whether the
// RTP payload begins a random-access picture (IDR/CRA), including FU starts.
func (rf *RTPForwarder) updateHEVCCache(seq uint16, payload []byte) bool {
	if len(payload) < 2 {
		return false
	}

	nalType := (payload[0] >> 1) & 0x3f
	switch nalType {
	case 32, 33, 34:
		rf.cacheHEVCParameterSet(nalType, payload)
		return false
	case 19, 20, 21:
		return true
	case 48: // Aggregation Packet (RFC 7798)
		randomAccess := false
		for off := 2; off+2 <= len(payload); {
			sz := int(payload[off])<<8 | int(payload[off+1])
			off += 2
			if sz < 2 || off+sz > len(payload) {
				break
			}
			nalu := payload[off : off+sz]
			t := (nalu[0] >> 1) & 0x3f
			if t == 32 || t == 33 || t == 34 {
				rf.cacheHEVCParameterSet(t, nalu)
			}
			if t == 19 || t == 20 || t == 21 {
				randomAccess = true
			}
			off += sz
		}
		return randomAccess
	case 49: // Fragmentation Unit (RFC 7798)
		if len(payload) < 3 {
			return false
		}

		start := payload[2]&0x80 != 0
		end := payload[2]&0x40 != 0
		fuType := payload[2] & 0x3f

		if start {
			rf.hevcFUType = fuType
			rf.hevcFUActive = fuType == 32 || fuType == 33 || fuType == 34
			rf.hevcFUSeq = seq
			rf.hevcFUSeqValid = true

			if rf.hevcFUActive {
				rf.hevcFUBuf = append(rf.hevcFUBuf[:0],
					payload[0]&0x81|((fuType<<1)&0x7e), payload[1])
				rf.hevcFUBuf = append(rf.hevcFUBuf, payload[3:]...)
			} else {
				rf.hevcFUBuf = nil
			}
		} else {
			if !rf.hevcFUActive || !rf.hevcFUSeqValid ||
				seq != rf.hevcFUSeq+1 || rf.hevcFUType != fuType {
				rf.hevcFUActive = false
				rf.hevcFUSeqValid = false
				rf.hevcFUBuf = nil
				return false
			}
			rf.hevcFUSeq = seq
			rf.hevcFUBuf = append(rf.hevcFUBuf, payload[3:]...)
		}

		randomAccess := start && (fuType == 19 || fuType == 20 || fuType == 21)
		if end {
			if rf.hevcFUActive {
				rf.cacheHEVCParameterSet(fuType, rf.hevcFUBuf)
			}
			rf.hevcFUActive = false
			rf.hevcFUSeqValid = false
			rf.hevcFUBuf = nil
		}

		return randomAccess
	default:
		return false
	}
}

func (rf *RTPForwarder) cacheHEVCParameterSet(nalType byte, nalu []byte) {
	copyNAL := append([]byte(nil), nalu...)
	switch nalType {
	case 32:
		rf.hevcVPS = copyNAL
	case 33:
		rf.hevcSPS = copyNAL
	case 34:
		rf.hevcPPS = copyNAL
	}
}

func (rf *RTPForwarder) ForwardAudioPacket(packet *rtp.Packet) {
	rf.mutex.Lock()

	if len(rf.clients) == 0 {
		rf.mutex.Unlock()
		return
	}

	var deadClients []string
	now := time.Now()

	for sessionID, client := range rf.clients {
		client.lastActivity.Store(now.UnixNano())

		if client.audioSSRC == 0 {
			client.audioSSRC = newRTPSSRC()
		}

		if !client.audioSeqInit {
			client.nextAudioSeq = newRTPSequence()
			client.audioSeqInit = true
		}

		out := *packet
		out.SSRC = client.audioSSRC
		out.SequenceNumber = client.nextAudioSeq
		client.nextAudioSeq++

		data, err := out.Marshal()
		if err != nil {
			core.Logger.Error().
				Err(err).
				Msg("Error marshaling audio RTP packet")
			continue
		}

		var writeErr error

		if client.transportMode == TransportUDP {
			if client.audioConn != nil {
				_, writeErr = client.audioConn.Write(data)
			}
		} else if client.transportMode == TransportTCP {
			if client.tcpConn != nil {
				writeErr = rf.sendInterleavedRTP(
					client.tcpConn,
					client.audioRTPChannel,
					data,
				)
			}
		}

		if writeErr != nil {
			if isDeadClientError(writeErr) {
				deadClients = append(deadClients, sessionID)
			} else {
				core.Logger.Error().
					Err(writeErr).
					Msgf(
						"Error forwarding audio packet to RTP client %s",
						sessionID,
					)
			}
			continue
		}

		if !rf.audioFirstLogged.Swap(true) {
			if client.transportMode == TransportUDP {
				core.Logger.Trace().Msgf(
					"Successfully sent first audio packet to UDP client %s on port %d",
					sessionID,
					client.audioRTPPort,
				)
			} else {
				core.Logger.Trace().Msgf(
					"Successfully sent first audio packet to TCP client %s on channel %d",
					sessionID,
					client.audioRTPChannel,
				)
			}
		}
	}

	for _, sessionID := range deadClients {
		if client, ok := rf.clients[sessionID]; ok {
			rf.closeClient(client)
			delete(rf.clients, sessionID)
			core.Logger.Debug().
				Msgf("Removing dead audio client %s", sessionID)
		}
	}

	rf.mutex.Unlock()
}

func isDeadClientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "connection reset")
}

func (rf *RTPForwarder) Stop() {
	rf.videoSSRC.Store(0)
	rf.audioSSRC.Store(1)
	rf.videoFirstLogged.Store(false)
	rf.audioFirstLogged.Store(false)

	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	rf.hevcVPS = nil
	rf.hevcSPS = nil
	rf.hevcPPS = nil
	rf.hevcFUType = 0
	rf.hevcFUBuf = nil
	rf.hevcFUActive = false
	rf.hevcFUSeq = 0
	rf.hevcFUSeqValid = false

	for _, client := range rf.clients {
		if client.transportMode == TransportUDP {
			if client.videoConn != nil {
				_ = client.videoConn.Close()
			}
			if client.audioConn != nil {
				_ = client.audioConn.Close()
			}
			if client.backchannelListener != nil {
				_ = client.backchannelListener.Close()
			}
			if client.backchannelRTCPListener != nil {
				_ = client.backchannelRTCPListener.Close()
			}
		} else if client.tcpConn != nil {
			_ = client.tcpConn.Close()
		}
	}
	rf.clients = make(map[string]*RTPClient)

	core.Logger.Trace().Msg("RTPForwarder stopped and all clients cleared")
}

func (rf *RTPForwarder) GetClientCount() int {
	rf.mutex.RLock()
	defer rf.mutex.RUnlock()
	return len(rf.clients)
}

func (rf *RTPForwarder) CleanupInactiveClients(timeout time.Duration) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	now := time.Now()
	var toRemove []string

	for sessionID, client := range rf.clients {
		if now.Sub(time.Unix(0, client.lastActivity.Load())) > timeout {
			toRemove = append(toRemove, sessionID)
		}
	}

	for _, sessionID := range toRemove {
		if client, exists := rf.clients[sessionID]; exists {
			if client.transportMode == TransportUDP {
				if client.videoConn != nil {
					_ = client.videoConn.Close()
				}
				if client.audioConn != nil {
					_ = client.audioConn.Close()
				}
				if client.backchannelListener != nil {
					_ = client.backchannelListener.Close()
				}
				if client.backchannelRTCPListener != nil {
					_ = client.backchannelRTCPListener.Close()
				}
			} else if client.tcpConn != nil {
				_ = client.tcpConn.Close()
			}
			delete(rf.clients, sessionID)
			core.Logger.Trace().Msgf("Cleaned up inactive RTP client %s", sessionID)
		}
	}
}

func (rf *RTPForwarder) handleUDPBackchannelRTP(sessionID string, listener *net.UDPConn) {
	defer listener.Close()

	buffer := make([]byte, 1500)

	for {
		n, _, err := listener.ReadFromUDP(buffer)
		if err != nil {
			if !strings.Contains(err.Error(), "closed") {
				core.Logger.Error().Err(err).Msgf("Error reading UDP RTP backchannel for client %s", sessionID)
			}
			break
		}

		// Parse RTP packet
		packet := &rtp.Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			continue
		}

		// Forward to WebRTC bridge
		if rf.OnBackchannelAudio != nil {
			rf.OnBackchannelAudio(packet)
		}
	}
}

func (rf *RTPForwarder) handleUDPBackchannelRTCP(listener *net.UDPConn) {
	defer listener.Close()

	buffer := make([]byte, 1500)

	for {
		_, _, err := listener.ReadFromUDP(buffer)
		if err != nil {
			// Ignore
			break
		}

		// Simply discard RTCP packets
	}
}

func (rf *RTPForwarder) sendInterleavedRTP(conn net.Conn, channel byte, rtpData []byte) error {
	var header [4]byte
	header[0] = '$'
	header[1] = channel
	header[2] = byte(len(rtpData) >> 8)
	header[3] = byte(len(rtpData))

	_, err := conn.Write(append(header[:], rtpData...))
	return err
}
