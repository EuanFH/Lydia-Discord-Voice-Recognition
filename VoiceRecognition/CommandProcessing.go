package VoiceRecognition

import (
	"DiscordVoiceRecognition/Config"
	"DiscordVoiceRecognition/RasaNLU"
	"bytes"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	texttospeechpb "google.golang.org/genproto/googleapis/cloud/texttospeech/v1"
	"io/ioutil"
	"layeh.com/gopus"
	"net/http"
	"time"
)

type UserCommand struct {
	UserId         string            `json:"userid"`
	GuildId        string            `json:"guildid"`
	VoiceChannelId string            `json:"voicechannelid"`
	Intent         RasaNLU.Intent    `json:"intent"`
	Entities       map[string]string `json:"entities"`
}

type RemoteBotResponse struct {
	Text       string `json:"text"`
	Understood bool   `json:"understood"`
	Callback   string `json:"callback"`
}

func commandProcessing(userId string, command string, voice *discordgo.VoiceConnection) chan bool {
	commandProcessed := make(chan bool)
	go func() {
		config := Config.LoadConfig()
		response := "sorry i didn't understand"
		//rasa
		parserResponse, err := RasaNLU.Parse(command, config.Rasa.Project)
		if err != nil {
			zap.S().Warn(err)
			commandProcessed <- true
			return
		}
		userCommand := newUserCommand(userId, parserResponse)
		//remote bot
		remoteBotResponse, err := sendUserCommandToRemoteBot(userCommand)
		if err != nil {
			zap.S().Warn(err)
			commandProcessed <- true
			return
		}
		if remoteBotResponse.Text != "" || remoteBotResponse.Understood {
			zap.S().Info("Command understood by remote bot")
			response = remoteBotResponse.Text
		}
		//response
		responseWave, err := textToSpeech(response)
		if err != nil {
			zap.S().Warn(err)
			commandProcessed <- true
			return
		}
		zap.S().Info("Reading out response")
		if err := playWaveAudio(responseWave, voice); err != nil {
			zap.S().Warn(err)
			commandProcessed <- true
			return
		}
		zap.S().Info("Finished reading response")
		//callback remote bot
		if remoteBotResponse.Callback != "" {
			zap.S().Infof("callback to %s", remoteBotResponse.Callback)
			client := http.Client{
				Timeout: time.Duration(5 * time.Second),
			}
			client.Get(remoteBotResponse.Callback)
			zap.S().Infof("finished callback to %s", remoteBotResponse.Callback)
		}
		commandProcessed <- true
		zap.S().Info("Finished command processing")
		return
	}()
	return commandProcessed
}

func textToSpeech(text string) ([]byte, error) {
	ctx := context.Background()

	client, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	req := texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},

		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-GB",
			Name:         "en-GB-Wavenet-A",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_FEMALE,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16,
			SampleRateHertz: 48000,
		},
	}

	resp, err := client.SynthesizeSpeech(ctx, &req)
	if err != nil {
		return nil, err
	}
	return resp.AudioContent, nil
}

/*
func playWaveAudio(wave []byte, voice *discordgo.VoiceConnection) error {
	waveNoHeader := wave[44:]
	pcmMono, err := byteSliceToInt16Slice(waveNoHeader)
	if err != nil {
		return err
	}
	pcmStereo := convertMonoToStero(pcmMono)
	pcmFrames := pcmSliceToPCMFrameSlice(pcmStereo)
	opusEncoder, err := opus.NewEncoder(discordSampleRate, 2, opus.AppVoIP)
	if err != nil {
		return err
	}
	//discord default bitrate
	opusEncoder.SetBitrate(64 * 1000)
	voice.Speaking(true)
	for i := 0; i < len(pcmFrames); i++ {
		opusFrame, err := encodePCMFrameToOpusBytes(pcmFrames[i], opusEncoder)
		if err != nil {
			log.Printf("failed to encode opus frame: %s", err)
			continue
		}
		voice.OpusSend <- opusFrame
	}
	voice.Speaking(false)
	return nil
}
*/

func playWaveAudio(wave []byte, voice *discordgo.VoiceConnection) error {
	waveNoHeader := wave[44:]
	pcmMono, err := byteSliceToInt16Slice(waveNoHeader)
	if err != nil {
		return err
	}
	pcmStereo := convertMonoToStero(pcmMono)
	pcmFrames := pcmSliceToPCMFrameSlice(pcmStereo)
	opusEncoder, err := gopus.NewEncoder(48000, 2, gopus.Audio)
	if err != nil {
		return err
	}
	//discord default bitrate
	voice.Speaking(true)
	for i := 0; i < len(pcmFrames); i++ {
		opusFrame, err := encodePCMFrameToOpusBytes(pcmFrames[i], opusEncoder)
		if err != nil {
			zap.S().Warn("failed to encode opus frame: %s", err)
			continue
		}
		voice.OpusSend <- opusFrame
	}
	voice.Speaking(false)
	return nil
}

/*
func encodePCMFrameToOpusBytes(pcm []int16, opusEncoder *opus.Encoder) ([]byte, error) {
	frameSize := len(pcm)
	frameSizeMs := float32(frameSize) / 2 * 1000 / discordSampleRate
	switch frameSizeMs {
	case 2.5, 5, 10, 20, 40, 60:
		// Good.
	default:
		return nil, fmt.Errorf("illegal frame size: %d bytes (%f ms)", frameSize, frameSizeMs)
	}
	data := make([]byte, 1000)
	n, err := opusEncoder.Encode(pcm, data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}
*/
func encodePCMFrameToOpusBytes(pcm []int16, opusEncoder *gopus.Encoder) ([]byte, error) {
	frameSize := len(pcm)
	frameSizeMs := float32(frameSize) / 2 * 1000 / discordSampleRate
	switch frameSizeMs {
	case 2.5, 5, 10, 20, 40, 60:
		// Good.
	default:
		return nil, fmt.Errorf("illegal frame size: %d bytes (%f ms)", frameSize, frameSizeMs)
	}
	opusData, err := opusEncoder.Encode(pcm, 960, (960*2)*2)
	if err != nil {
		return nil, fmt.Errorf("failed to encode opus: %s", err)
	}
	return opusData, nil
}

func sendUserCommandToRemoteBot(userCommand *UserCommand) (*RemoteBotResponse, error) {
	config := Config.LoadConfig()
	userCommandJson, err := json.Marshal(userCommand)
	if err != nil {
		return nil, err
	}
	requestBody := bytes.NewReader(userCommandJson)
	timeout := time.Duration(10 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Post(config.RemoteBot.Address, "application/json", requestBody)
	if resp == nil {
		return nil, errors.New("remote bot failed to respond")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		body := string(bodyBytes)
		return nil, errors.New(fmt.Sprintf("Non 200 status:\nStatus: %s\nBody: %s", resp.Status, body))
	}
	remoteBotResponse := &RemoteBotResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&remoteBotResponse); err != nil {
		return nil, err
	}
	return remoteBotResponse, nil
}

func newUserCommand(userid string, parserResponse *RasaNLU.ParserResponse) *UserCommand {
	config := Config.LoadConfig()
	userCommand := UserCommand{UserId: userid, GuildId: config.Discord.Guild, VoiceChannelId: config.Discord.VoiceChannel, Intent: parserResponse.Intent}
	entities := make(map[string]string)
	for _, entity := range parserResponse.Entities {
		entities[entity.Entity] = entity.Value
	}
	userCommand.Entities = entities
	return &userCommand
}
