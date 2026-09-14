package rtsp

import (
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
}

func NewRTPForwarder() *RTPForwarder {
	rf := &RTPForwarder{
		clients: make(map[string]*RTPClient),
	}
	rf.audioSSRC.Store(1)
	return rf
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
	rf.mutex.RLock()
	if len(rf.clients) == 0 {
		rf.mutex.RUnlock()
		return
	}

	data, err := packet.Marshal()
	if err != nil {
		rf.mutex.RUnlock()
		core.Logger.Error().Err(err).Msg("Error marshaling video RTP packet")
		return
	}

	var deadClients []string
	now := time.Now()

	for sessionID, client := range rf.clients {
		client.lastActivity.Store(now.UnixNano())

		var writeErr error
		if client.transportMode == TransportUDP {
			if client.videoConn != nil {
				_, writeErr = client.videoConn.Write(data)
			}
		} else if client.transportMode == TransportTCP && client.tcpConn != nil {
			writeErr = rf.sendInterleavedRTP(client.tcpConn, client.videoRTPChannel, data)
		}

		if writeErr != nil {
			if isDeadClientError(writeErr) {
				deadClients = append(deadClients, sessionID)
			} else {
				core.Logger.Error().Err(writeErr).Msgf("Error forwarding video packet to RTP client %s", sessionID)
			}
			continue
		}

		if !rf.videoFirstLogged.Swap(true) {
			if client.transportMode == TransportUDP {
				core.Logger.Trace().Msgf("Successfully sent first video packet to UDP client %s on port %d", sessionID, client.videoRTPPort)
			} else {
				core.Logger.Trace().Msgf("Successfully sent first video packet to TCP client %s on channel %d", sessionID, client.videoRTPChannel)
			}
		}
	}

	rf.mutex.RUnlock()

	for _, sessionID := range deadClients {
		core.Logger.Debug().Msgf("Removing dead video client %s", sessionID)
		rf.RemoveClient(sessionID)
	}
}

func (rf *RTPForwarder) ForwardAudioPacket(packet *rtp.Packet) {
	rf.mutex.RLock()
	if len(rf.clients) == 0 {
		rf.mutex.RUnlock()
		return
	}

	data, err := packet.Marshal()
	if err != nil {
		rf.mutex.RUnlock()
		core.Logger.Error().Err(err).Msg("Error marshaling audio RTP packet")
		return
	}

	var deadClients []string
	now := time.Now()

	for sessionID, client := range rf.clients {
		client.lastActivity.Store(now.UnixNano())

		var writeErr error
		if client.transportMode == TransportUDP {
			if client.audioConn != nil {
				_, writeErr = client.audioConn.Write(data)
			}
		} else if client.transportMode == TransportTCP && client.tcpConn != nil {
			writeErr = rf.sendInterleavedRTP(client.tcpConn, client.audioRTPChannel, data)
		}

		if writeErr != nil {
			if isDeadClientError(writeErr) {
				deadClients = append(deadClients, sessionID)
			} else {
				core.Logger.Error().Err(writeErr).Msgf("Error forwarding audio packet to RTP client %s", sessionID)
			}
			continue
		}

		if !rf.audioFirstLogged.Swap(true) {
			if client.transportMode == TransportUDP {
				core.Logger.Trace().Msgf("Successfully sent first audio packet to UDP client %s on port %d", sessionID, client.audioRTPPort)
			} else {
				core.Logger.Trace().Msgf("Successfully sent first audio packet to TCP client %s on channel %d", sessionID, client.audioRTPChannel)
			}
		}
	}

	rf.mutex.RUnlock()

	for _, sessionID := range deadClients {
		core.Logger.Debug().Msgf("Removing dead audio client %s", sessionID)
		rf.RemoveClient(sessionID)
	}
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

	buf := net.Buffers{header[:], rtpData}
	_, err := buf.WriteTo(conn)
	return err
}
