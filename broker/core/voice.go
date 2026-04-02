// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

package core

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/pion/webrtc/v4"
	wstypes "github.com/srschreiber/nito/shared/websocket_types"
)

const (
	voiceClockRate   = 48000
	voiceChannels    = 2   // Opus SDP spec uses 2 even for mono streams
	voicePayloadType = 111 // standard dynamic payload type for Opus
	voiceFmtp        = "minptime=10;useinbandfec=1"
)

// VoiceRoom holds all active participants in a voice call for one room.
type VoiceRoom struct {
	ID           string
	mu           sync.RWMutex
	Participants map[string]*VoiceParticipant // userID → participant
}

// VoiceParticipant represents a single WebRTC peer connected to the broker for voice.
type VoiceParticipant struct {
	UserID string
	PC     *webrtc.PeerConnection

	mu sync.RWMutex

	// Tracks this participant is publishing to the broker.
	Incoming map[string]*webrtc.TrackRemote

	// Local tracks the broker sends out to this participant, keyed by source userID.
	Outgoing map[string]*webrtc.TrackLocalStaticRTP

	// RTPSenders for Outgoing tracks; required for RemoveTrack on leave.
	Senders map[string]*webrtc.RTPSender
}

func newVoicePC() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeOpus,
			ClockRate:   voiceClockRate,
			Channels:    voiceChannels,
			SDPFmtpLine: voiceFmtp,
		},
		PayloadType: voicePayloadType,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("register codec: %w", err)
	}
	se := webrtc.SettingEngine{}
	if err := se.SetEphemeralUDPPortRange(10000, 10100); err != nil {
		return nil, fmt.Errorf("set udp port range: %w", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return nil, err
	}
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("voice: connection state -> %s", s)
	})
	return pc, nil
}

func (b *Broker) getOrCreateVoiceRoom(roomID string) *VoiceRoom {
	b.voiceMu.Lock()
	defer b.voiceMu.Unlock()
	if room, ok := b.voiceRooms[roomID]; ok {
		return room
	}
	room := &VoiceRoom{ID: roomID, Participants: make(map[string]*VoiceParticipant)}
	b.voiceRooms[roomID] = room
	return room
}

func (b *Broker) getVoiceParticipant(roomID, userID string) (*VoiceRoom, *VoiceParticipant) {
	b.voiceMu.RLock()
	room := b.voiceRooms[roomID]
	b.voiceMu.RUnlock()
	if room == nil {
		return nil, nil
	}
	room.mu.RLock()
	p := room.Participants[userID]
	room.mu.RUnlock()
	return room, p
}

// voiceJoin processes a new participant joining a room's voice call.
// sdpOffer is the WebRTC SDP offer from the client; returns the broker's SDP answer.
func (b *Broker) voiceJoin(userID, roomID, sdpOffer string) (string, error) {
	room := b.getOrCreateVoiceRoom(roomID)

	pc, err := newVoicePC()
	if err != nil {
		return "", fmt.Errorf("new peer connection: %w", err)
	}

	participant := &VoiceParticipant{
		UserID:   userID,
		PC:       pc,
		Incoming: make(map[string]*webrtc.TrackRemote),
		Outgoing: make(map[string]*webrtc.TrackLocalStaticRTP),
		Senders:  make(map[string]*webrtc.RTPSender),
	}

	// Wire up bidirectional outbound tracks with every existing participant.
	room.mu.Lock()
	for _, existing := range room.Participants {
		// Track the broker will relay existing participant's audio → new participant.
		outForNew, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeOpus, ClockRate: voiceClockRate, Channels: voiceChannels,
		}, existing.UserID+"-pcm", "stream-"+existing.UserID)
		if err != nil {
			room.mu.Unlock()
			pc.Close()
			return "", fmt.Errorf("new track for %s: %w", existing.UserID, err)
		}
		senderForNew, err := pc.AddTrack(outForNew)
		if err != nil {
			room.mu.Unlock()
			pc.Close()
			return "", fmt.Errorf("add track for %s: %w", existing.UserID, err)
		}
		participant.Outgoing[existing.UserID] = outForNew
		participant.Senders[existing.UserID] = senderForNew

		// Track the broker will relay new participant's audio → existing participant.
		outForExisting, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeOpus, ClockRate: voiceClockRate, Channels: voiceChannels,
		}, userID+"-pcm", "stream-"+userID)
		if err != nil {
			room.mu.Unlock()
			pc.Close()
			return "", fmt.Errorf("new outbound track for existing %s: %w", existing.UserID, err)
		}
		existing.mu.Lock()
		senderForExisting, err := existing.PC.AddTrack(outForExisting)
		if err != nil {
			existing.mu.Unlock()
			room.mu.Unlock()
			pc.Close()
			return "", fmt.Errorf("add track to existing %s: %w", existing.UserID, err)
		}
		existing.Outgoing[userID] = outForExisting
		existing.Senders[userID] = senderForExisting
		existing.mu.Unlock()

		go b.renegotiateVoice(existing, roomID)
	}
	room.Participants[userID] = participant
	room.mu.Unlock()

	// When media arrives from this participant, forward it to everyone else in the room.
	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		participant.mu.Lock()
		participant.Incoming[remote.ID()] = remote
		participant.mu.Unlock()
		log.Printf("voice: %s published track %s in room %s", userID, remote.ID(), roomID)
		go b.forwardVoiceTrack(room, userID, remote)
	})

	// Set the client's offer and create our answer.
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpOffer,
	}); err != nil {
		return "", fmt.Errorf("set remote description: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("create answer: %w", err)
	}
	gatherDone := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("set local description: %w", err)
	}
	<-gatherDone

	return pc.LocalDescription().SDP, nil
}

// voiceLeave removes a participant from a room and cleans up their peer connection.
func (b *Broker) voiceLeave(userID, roomID string) {
	b.voiceMu.RLock()
	room := b.voiceRooms[roomID]
	b.voiceMu.RUnlock()
	if room == nil {
		return
	}

	room.mu.Lock()
	participant := room.Participants[userID]
	if participant == nil {
		room.mu.Unlock()
		return
	}
	delete(room.Participants, userID)

	// Remove the leaving participant's outbound track from each remaining participant.
	for _, p := range room.Participants {
		p.mu.Lock()
		sender := p.Senders[userID]
		if sender != nil {
			if err := p.PC.RemoveTrack(sender); err != nil {
				log.Printf("voice: remove track %s from %s: %v", userID, p.UserID, err)
			}
			delete(p.Senders, userID)
			delete(p.Outgoing, userID)
		}
		p.mu.Unlock()
		go b.renegotiateVoice(p, roomID)
	}
	remaining := len(room.Participants)
	room.mu.Unlock()

	_ = participant.PC.Close()
	log.Printf("voice: %s left room %s", userID, roomID)

	if remaining == 0 {
		b.voiceMu.Lock()
		delete(b.voiceRooms, roomID)
		b.voiceMu.Unlock()
	}
}

// voiceRenegAnswer applies a renegotiation answer from a client.
func (b *Broker) voiceRenegAnswer(userID, roomID, sdpAnswer string) error {
	_, p := b.getVoiceParticipant(roomID, userID)
	if p == nil {
		return fmt.Errorf("participant %s not in room %s", userID, roomID)
	}
	return p.PC.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdpAnswer,
	})
}

// forwardVoiceTrack reads RTP from a remote track and writes it to all other participants'
// corresponding outbound track for this sender.
func (b *Broker) forwardVoiceTrack(room *VoiceRoom, senderUserID string, remote *webrtc.TrackRemote) {
	for {
		pkt, _, err := remote.ReadRTP()
		if err != nil {
			log.Printf("voice: forwardVoiceTrack read (%s/%s): %v", room.ID, senderUserID, err)
			return
		}
		room.mu.RLock()
		for uid, p := range room.Participants {
			if uid == senderUserID {
				continue
			}
			p.mu.RLock()
			out := p.Outgoing[senderUserID]
			p.mu.RUnlock()
			if out != nil {
				if err := out.WriteRTP(pkt); err != nil {
					log.Printf("voice: forward to %s: %v", uid, err)
				}
			}
		}
		room.mu.RUnlock()
	}
}

// renegotiateVoice sends a fresh SDP offer to an existing participant after their
// track set has changed (new participant joined or existing one left).
func (b *Broker) renegotiateVoice(participant *VoiceParticipant, roomID string) {
	offer, err := participant.PC.CreateOffer(nil)
	if err != nil {
		log.Printf("voice: renegotiate %s: create offer: %v", participant.UserID, err)
		return
	}
	gatherDone := webrtc.GatheringCompletePromise(participant.PC)
	if err := participant.PC.SetLocalDescription(offer); err != nil {
		log.Printf("voice: renegotiate %s: set local desc: %v", participant.UserID, err)
		return
	}
	<-gatherDone

	payload, err := json.Marshal(wstypes.VoiceOfferPayload{
		RoomID:   roomID,
		SDPOffer: participant.PC.LocalDescription().SDP,
	})
	if err != nil {
		log.Printf("voice: renegotiate %s: marshal: %v", participant.UserID, err)
		return
	}
	b.sendToClient(participant.UserID, wstypes.RPCVoiceOffer, payload)
}
