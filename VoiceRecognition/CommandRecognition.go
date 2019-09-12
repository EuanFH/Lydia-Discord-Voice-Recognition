package VoiceRecognition

import (
	speech "cloud.google.com/go/speech/apiv1"
	"context"
	"go.uber.org/zap"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
	"gopkg.in/hraban/opus.v2"
	"io"
	"time"
)

type CommandRecognition struct {
	client                   *speech.Client
	streamingRecognizeClient speechpb.Speech_StreamingRecognizeClient
	VoiceInfoRecv            chan *VoiceInfo
	opusDecoder              *opus.Decoder
	commandNotify            chan<- string
	close                    chan bool
}

func createCommandRecognition(commandNotify chan<- string) *CommandRecognition {
	opusDecoder, err := opus.NewDecoder(48000, 2)
	if err != nil {
		zap.S().Fatalf("Failed to create opus decoder for command recognition")
	}
	ctx := context.Background()

	client, err := speech.NewClient(ctx)
	if err != nil {
		zap.S().Fatal(err)
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		zap.S().Fatal(err)
	}

	if err := stream.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:        speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz: discordSampleRate,
					LanguageCode:    "en-GB",
				},
				SingleUtterance: true,
			},
		},
	}); err != nil {
		zap.S().Fatal(err)
	}

	//dont know if i need a buffer for the packets
	//should decode fast enough
	cr := &CommandRecognition{
		client:                   client,
		streamingRecognizeClient: stream,
		VoiceInfoRecv:            make(chan *VoiceInfo, 1000),
		opusDecoder:              opusDecoder,
		commandNotify:            commandNotify,
		close:                    make(chan bool),
	}
	go cr.voiceRecv()
	go cr.readResponse()
	return cr

}

func (cr *CommandRecognition) voiceRecv() {
	ticker := time.NewTicker(20 * time.Millisecond)
	timeout := time.NewTimer(15 * time.Second)
	defer timeout.Stop()
	speaking := false
	for {
		select {
		case voiceInfo := <-cr.VoiceInfoRecv:
			pcm := make([]int16, frameSizeStereo)
			if !voiceInfo.speaking {
				speaking = false
				continue
			}
			speaking = true
			if _, err := cr.opusDecoder.Decode(voiceInfo.packet.Opus, pcm); err != nil {
				zap.S().Info("Failed to decode voice packet for command recognition")
				continue
			}
			cr.sendVoice(pcm)

		case <-ticker.C:
			if speaking {
				continue
			}
			cr.sendVoice(silencePacketPCM)
		case <-timeout.C:
			if err := cr.streamingRecognizeClient.CloseSend(); err != nil {
				zap.S().Infof("Could not close stream: %v", err)
			}
			if err := cr.client.Close(); err != nil {
				zap.S().Infof("Could not close client: %v", err)
			}
			return
		case <-cr.close:
			if err := cr.streamingRecognizeClient.CloseSend(); err != nil {
				zap.S().Fatalf("Could not close stream: %v", err)
			}
			if err := cr.client.Close(); err != nil {
				zap.S().Fatalf("Could not close client: %v", err)
			}
			return
		}
	}
}

func (cr *CommandRecognition) readResponse() {
	for {
		resp, err := cr.streamingRecognizeClient.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			zap.S().Infof("Cannot stream results: %v", err)
			cr.commandNotify <- ""
			cr.close <- true
			return
		}
		if err := resp.Error; err != nil {
			// Workaround while the API doesn't give a more informative error.
			if err.Code == 3 || err.Code == 11 {
				zap.S().Info("WARNING: Speech recognition request exceeded limit of 60 seconds.")

			}
			zap.S().Infof("Could not recognize: %v", err)
			cr.commandNotify <- ""
			cr.close <- true
			return
		}
		for _, result := range resp.Results {
			cr.commandNotify <- result.Alternatives[0].Transcript
			cr.close <- true
			return
		}
	}
}

func (cr *CommandRecognition) sendVoice(pcm []int16) {
	//could mabye use a two channel configuration for google speech recognition removing the need to convert to mono
	pcmMono := convertPCMToMono(pcm)
	pcmBytes, err := int16SliceToByteSlice(pcmMono)
	if err != nil {
		zap.S().Warn("Failed to convert int16 pcm to byte slice for command recognition")
		return
	}
	if err := cr.streamingRecognizeClient.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
			AudioContent: pcmBytes,
		},
	}); err != nil {
		zap.S().Warnf("Could not send audio: %v", err)
	}
}
