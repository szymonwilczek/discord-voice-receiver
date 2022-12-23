package main

import (
	_ "encoding/json"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
	"path/filepath"
	"time"
	_ "time"
)

// Variables used for command line parameters
var (
	Token     string
	ChannelID string
	GuildID   string
)

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.StringVar(&GuildID, "g", "", "Guild in which voice channel exists")
	flag.StringVar(&ChannelID, "c", "", "Voice channel to connect to")
	flag.Parse()
}

func createPionRTPPacket(p *discordgo.Packet) *rtp.Packet {
	return &rtp.Packet{
		Header: rtp.Header{
			Version: 2,
			// From Discord Docs
			PayloadType:    0x78,
			SequenceNumber: p.Sequence,
			Timestamp:      p.Timestamp,
			SSRC:           p.SSRC,
		},
		Payload: p.Opus,
	}
}

func handleVoice(c chan *discordgo.Packet) {
	startTime := time.Now()
	files := make(map[uint32]media.Writer)
	for p := range c {
		file, ok := files[p.SSRC]
		if !ok {
			var err error
			//get current date and time in format DD-MM-YYYY-HH-MM-SS
			now := time.Now()
			formattedTime := now.Format("02-01-2006 15-04-05")

			filePath := filepath.Join("recordings", fmt.Sprintf("%s.ogg", formattedTime))
			file, err = oggwriter.New(filePath, 48000, 2)
			if err != nil {
				fmt.Printf("Failed to created file %s.ogg: %v\n", formattedTime, err)
				return
			}
			files[p.SSRC] = file
		}

		durationTime := time.Since(startTime)
		rtp := createPionRTPPacket(p)
		rtp.Header.Timestamp = uint32(durationTime.Seconds() * 48000)
		err := file.WriteRTP(rtp)
		if err != nil {
			fmt.Printf("Failed to created file %d.ogg: %v\n", p.SSRC, err)
		}
	}

}

func main() {
	s, err := discordgo.New("Bot " + Token)
	fmt.Println("Bot online!")
	if err != nil {
		fmt.Println("An error occurred establishing a connection to the session:", err)
		return
	}
	defer s.Close()

	// We only really care about receiving voice state updates and message create events.
	s.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMessages)

	// Register a message create event handler.
	s.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {

		if m.Content == "!start" {
			fmt.Println("🎥 Starting recording")
			// Join the voice channel and start recording as before.
			v, err := s.ChannelVoiceJoin(GuildID, ChannelID, true, false)
			if err != nil {
				fmt.Println("Failed to join voice channel:", err)
				return
			}
			handleVoice(v.OpusRecv)
		}

		// Check if the message content is "!stop". If it is, close the channel.
		if m.Content == "!stop" {
			err := s.VoiceConnections[GuildID].Disconnect()
			if err != nil {
				fmt.Println("An error occurred while exiting the channel:", err)
				return
			}

			//close the file
			files := make(map[uint32]media.Writer)
			for _, file := range files {
				err := file.Close()
				if err != nil {
					fmt.Println("An error occurred while closing the file:", err)
					return
				}
			}

			fmt.Println("🟥 Recording completed")
		}
	})

	err = s.Open()
	if err != nil {
		fmt.Println("An error occurred reconnecting the session:", err)
		return
	}

	// Wait indefinitely.
	<-make(chan struct{})
}
