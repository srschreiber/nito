package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/oto/v2"
	media "github.com/pion/mediadevices"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"golang.org/x/sync/errgroup"
)

/*
This is not used, but is what voice communication was modelled after
*/

const (
	sampleRate  = 48000
	channels    = 1
	frameMs     = 10                                         // 10 ms keeps one encrypted PCM frame under common MTU
	frameBytes  = sampleRate * channels * 2 * frameMs / 1000 // 960 bytes @ 48k mono s16le
	payloadType = 96
	clockRate   = 48000
	mimeType    = "audio/x-pcm-demo"
)

func main() {
	// Application-layer E2EE key. Broker should never know plaintext audio.
	appKey := make([]byte, 32) // AES-256
	if _, err := rand.Read(appKey); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("demo AES key: %x\n", appKey)

	aead, err := newAEAD(appKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create the four PeerConnections:
	//   clientA <-> brokerIn
	//   brokerOut <-> clientB
	pcA, err := newPC()
	if err != nil {
		log.Fatal(err)
	}
	defer pcA.Close()

	pcBrokerIn, err := newPC()
	if err != nil {
		log.Fatal(err)
	}
	defer pcBrokerIn.Close()

	pcBrokerOut, err := newPC()
	if err != nil {
		log.Fatal(err)
	}
	defer pcBrokerOut.Close()

	pcB, err := newPC()
	if err != nil {
		log.Fatal(err)
	}
	defer pcB.Close()

	// Sender track: client A writes encrypted PCM into RTP packets.
	sendTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{
			MimeType:  mimeType,
			ClockRate: clockRate,
			Channels:  channels,
		},
		"a-pcm",
		"stream-a",
	)
	if err != nil {
		log.Fatal(err)
	}
	if _, err = pcA.AddTrack(sendTrack); err != nil {
		log.Fatal(err)
	}

	// Broker outbound track: broker forwards encrypted RTP payload unchanged to B.
	brokerOutTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{
			MimeType:  mimeType,
			ClockRate: clockRate,
			Channels:  channels,
		},
		"b-pcm",
		"stream-b",
	)
	if err != nil {
		log.Fatal(err)
	}
	if _, err = pcBrokerOut.AddTrack(brokerOutTrack); err != nil {
		log.Fatal(err)
	}

	// B playback setup.
	otoCtx, ready, err := oto.NewContext(sampleRate, channels, oto.FormatSignedInt16LE)
	if err != nil {
		log.Fatal(err)
	}
	<-ready
	player := otoCtx.NewPlayer(bytes.NewReader(nil))
	_ = player // kept to show the context exists; we'll use a pipe instead

	pr, pw := io.Pipe()
	audioPlayer := otoCtx.NewPlayer(pr)
	audioPlayer.Play()
	defer audioPlayer.Close()

	// B receives RTP, decrypts the payload, and plays raw PCM.
	pcB.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Println("B: got track:", track.Codec().MimeType)

		go func() {
			for {
				pkt, _, err := track.ReadRTP()
				if err != nil {
					log.Printf("B read RTP: %v", err)
					return
				}

				plaintext, err := decryptFrame(aead, pkt.Payload)
				if err != nil {
					log.Printf("B decrypt frame: %v", err)
					continue
				}

				if _, err := pw.Write(plaintext); err != nil {
					log.Printf("B speaker write: %v", err)
					return
				}
			}
		}()
	})

	// Broker receives RTP from A and forwards it to B unchanged.
	pcBrokerIn.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Println("broker: got incoming track:", track.Codec().MimeType)

		go func() {
			for {
				pkt, _, err := track.ReadRTP()
				if err != nil {
					log.Printf("broker read RTP: %v", err)
					return
				}

				// Important:
				// - SRTP on A<->broker was already decrypted by Pion.
				// - pkt.Payload is still app-encrypted ciphertext.
				// - broker forwards payload unchanged.
				if err := brokerOutTrack.WriteRTP(pkt); err != nil {
					log.Printf("broker forward RTP: %v", err)
					return
				}
			}
		}()
	})

	// In-memory signaling: no REST here, just local SDP exchange to simulate setup.
	if err := signalPair(pcBrokerOut, pcB); err != nil {
		log.Fatal(err)
	}
	if err := signalPair(pcA, pcBrokerIn); err != nil {
		log.Fatal(err)
	}

	// Wait for connections to come up.
	waitConnected("A", pcA)
	waitConnected("brokerIn", pcBrokerIn)
	waitConnected("brokerOut", pcBrokerOut)
	waitConnected("B", pcB)

	fmt.Println("all peers connected; start capturing mic")

	// Capture mic PCM on A, encrypt per 10ms frame, send as RTP.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var g errgroup.Group
	g.Go(func() error {
		return captureEncryptAndSend(ctx, aead, sendTrack)
	})

	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}

func newPC() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}

	// Register a custom RTP codec so both Pion endpoints agree on the payload format.
	if err := m.RegisterCodec(
		webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  mimeType,
				ClockRate: clockRate,
				Channels:  channels,
			},
			PayloadType: payloadType,
		},
		webrtc.RTPCodecTypeAudio,
	); err != nil {
		return nil, fmt.Errorf("register codec: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("pc state -> %s", state.String())
	})

	return pc, nil
}

// signalPair performs in-memory SDP offer/answer exchange between two PeerConnections.
// in practice, this would be done over websocket
func signalPair(offerPC, answerPC *webrtc.PeerConnection) error {
	offer, err := offerPC.CreateOffer(nil)
	if err != nil {
		return fmt.Errorf("create offer: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(offerPC)

	if err := offerPC.SetLocalDescription(offer); err != nil {
		return fmt.Errorf("set offer local: %w", err)
	}
	<-gatherComplete

	if err := answerPC.SetRemoteDescription(*offerPC.LocalDescription()); err != nil {
		return fmt.Errorf("set answer remote: %w", err)
	}

	answer, err := answerPC.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}

	answerGatherComplete := webrtc.GatheringCompletePromise(answerPC)

	if err := answerPC.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("set answer local: %w", err)
	}
	<-answerGatherComplete

	if err := offerPC.SetRemoteDescription(*answerPC.LocalDescription()); err != nil {
		return fmt.Errorf("set offer remote: %w", err)
	}

	return nil
}

func waitConnected(name string, pc *webrtc.PeerConnection) {
	for {
		state := pc.ConnectionState()
		if state == webrtc.PeerConnectionStateConnected {
			fmt.Println(name, "connected")
			return
		}
		if state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateDisconnected {
			log.Fatalf("%s entered bad state: %s", name, state.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func captureEncryptAndSend(ctx context.Context, aead cipher.AEAD, track *webrtc.TrackLocalStaticRTP) error {
	stream, err := media.GetUserMedia(media.MediaStreamConstraints{
		Audio: func(c *media.MediaTrackConstraints) {},
	})
	if err != nil {
		return fmt.Errorf("get user media: %w", err)
	}

	tracks := stream.GetAudioTracks()
	if len(tracks) == 0 {
		return fmt.Errorf("no audio tracks found")
	}
	audioTrack := tracks[0]
	defer audioTrack.Close()

	reader := audioTrack.(*media.AudioTrack).NewReader(false)

	var seq uint32
	var ts uint32
	var pcmBuf bytes.Buffer

	ticker := time.NewTicker(frameMs * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		chunk, release, err := reader.Read()
		if err != nil {
			return fmt.Errorf("reader read: %w", err)
		}

		rawPCM, err := chunkToPCM16LE(chunk)
		release()
		if err != nil {
			return err
		}

		if _, err := pcmBuf.Write(rawPCM); err != nil {
			return err
		}

		for pcmBuf.Len() >= frameBytes {
			frame := make([]byte, frameBytes)
			if _, err := io.ReadFull(&pcmBuf, frame); err != nil {
				return err
			}

			ciphertext, err := encryptFrame(aead, frame)
			if err != nil {
				return err
			}

			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    payloadType,
					SequenceNumber: uint16(atomic.AddUint32(&seq, 1)),
					Timestamp:      ts,
					SSRC:           123456,
					Marker:         true,
				},
				Payload: ciphertext,
			}

			if err := track.WriteRTP(pkt); err != nil {
				return fmt.Errorf("write RTP: %w", err)
			}

			// 10 ms of 48k audio = 480 timestamp units for 1 channel audio clocked at 48kHz.
			ts += uint32(sampleRate * frameMs / 1000)
		}

		// Prevent a tight loop if the source delivers huge chunks.
		select {
		case <-ticker.C:
		default:
		}
	}
}

func chunkToPCM16LE(chunk any) ([]byte, error) {
	switch pcm := chunk.(type) {
	case *wave.Int16Interleaved:
		buf := make([]byte, len(pcm.Data)*2)
		for i, s := range pcm.Data {
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
		}
		return buf, nil

	case *wave.Float32Interleaved:
		buf := make([]byte, len(pcm.Data)*2)
		for i, s := range pcm.Data {
			v := int16(s * 32767)
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
		}
		return buf, nil

	default:
		return nil, fmt.Errorf("unsupported audio chunk type %T", chunk)
	}
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptFrame(aead cipher.AEAD, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// Wire format: nonce || ciphertext
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

func decryptFrame(aead cipher.AEAD, payload []byte) ([]byte, error) {
	ns := aead.NonceSize()
	if len(payload) < ns {
		return nil, fmt.Errorf("payload too short")
	}

	nonce := payload[:ns]
	ciphertext := payload[ns:]
	return aead.Open(nil, nonce, ciphertext, nil)
}
