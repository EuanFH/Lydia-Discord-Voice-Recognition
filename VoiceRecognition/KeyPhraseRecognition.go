package VoiceRecognition

import (
	"DiscordVoiceRecognition/Config"
	"bytes"
	"github.com/bwmarrin/discordgo"
	"github.com/xlab/pocketsphinx-go/sphinx"
	"github.com/zaf/resample"
	"go.uber.org/zap"
	"gopkg.in/hraban/opus.v2"
	"io"
)

//opusSilence is the silence packets sent by discord when a
//user starts and stop speaking
var opusSilence = []byte{0xF8, 0xFF, 0xFE}
var silencePacketPCM = make([]int16, frameSizeStereo)

const amountOfSamplesNeededToResample = 7680
const discordSampleRate = 48000
const sphinxSampleRate = 16000
const frameSizeStereo = 2 * 20 * 48000 / 1000

type SphinxListener struct {
	inSpeech   bool
	uttStarted bool
	dec        *sphinx.Decoder
}

type KeyPhraseRecognition struct {
	VoiceInfoRecv       chan *VoiceInfo
	keywordSpokenNotify chan KeywordSpokenNotify
	pcmBuffer           []int16
	opusDecoder         *opus.Decoder
	sphinxListener      SphinxListener
}

func createKeyPhraseRecognition(keywordSpokenNotify chan KeywordSpokenNotify) (*KeyPhraseRecognition, error) {
	//buffer is abritary just to stop blocking
	//the buffer might not be needed
	config := Config.LoadConfig()

	opusDecoder, err := opus.NewDecoder(48000, 2)
	if err != nil {
		return nil, err
	}

	sphinxConfig := sphinx.NewConfig(
		sphinx.LogFileOption(config.Sphinx.LogFile),
		sphinx.HMMDirOption(config.Sphinx.HMM),
		sphinx.DictFileOption(config.Sphinx.Dict),
		sphinx.KeywordsFileOption(config.Sphinx.KeywordsFile),
	)

	decoder, err := sphinx.NewDecoder(sphinxConfig)
	if err != nil {
		return nil, err
	}

	kr := &KeyPhraseRecognition{
		VoiceInfoRecv:       make(chan *VoiceInfo, 100),
		keywordSpokenNotify: keywordSpokenNotify,
		opusDecoder:         opusDecoder,
		sphinxListener:      SphinxListener{dec: decoder},
	}
	kr.sphinxListener.dec.StartUtt()
	kr.start()
	return kr, nil
}

//need second utterance check while listening to users voice command to check for keyword again to reset
//listening for canceling is also needed
func (kr *KeyPhraseRecognition) start() {
	go func() {
		for {
			voiceInfo, ok := <-kr.VoiceInfoRecv
			if !ok {
				return
			}
			if voiceInfo.speaking {
				if err := kr.decode(voiceInfo.packet); err != nil {
					zap.S().Info("failed to decode opus voice packet")
				}

			} else {
				//add silence so sphinx knows the user has stopped talking
				for i := 0; i < 20; i++ {
					kr.pcmBuffer = append(kr.pcmBuffer, silencePacketPCM...)
				}
				//if there is not enough audio to downsample add more silence till there is
				kr.fillBufferForDownsample()
			}
			keyphraseSpoken, err := kr.keyPhraseListing()
			if err != nil {
				zap.S().Infof("Failed to listen to command with error: %s", err)
			}
			if len(keyphraseSpoken) > 0 {
				kr.keywordSpokenNotify <- KeywordSpokenNotify{
					ssrc:      voiceInfo.packet.SSRC,
					keyPhrase: keyphraseSpoken,
				}
			}
		}

	}()
}

//Fills buffer with silence pcm to make it viable to downsample
func (kr *KeyPhraseRecognition) fillBufferForDownsample() {
	kr.pcmBuffer = append(kr.pcmBuffer, silencePacketPCM...)
	neededSamplesLength := amountOfSamplesNeededToResample - len(kr.pcmBuffer)
	if neededSamplesLength > 0 {
		numberOfSilenceFramesNeeded := neededSamplesLength / frameSizeStereo
		for i := 0; i < numberOfSilenceFramesNeeded; i++ {
			kr.pcmBuffer = append(kr.pcmBuffer, silencePacketPCM...)
		}
	}
}

func (kr *KeyPhraseRecognition) decode(packet *discordgo.Packet) error {
	pcmOrig := make([]int16, frameSizeStereo)

	_, err := kr.opusDecoder.Decode(packet.Opus, pcmOrig)
	if err != nil {
		return err
	}
	kr.pcmBuffer = append(kr.pcmBuffer, pcmOrig...)
	return nil
}

func (kr *KeyPhraseRecognition) keyPhraseListing() (string, error) {
	if len(kr.pcmBuffer) < amountOfSamplesNeededToResample {
		return "", nil
	}
	resampledPCM, err := resamplePCM(kr.pcmBuffer)
	if err != nil {
		return "", err
	}
	resampledMonoPCM := convertPCMToMono(resampledPCM)
	_, ok := kr.sphinxListener.dec.ProcessRaw(resampledMonoPCM, true, false)
	if !ok {
		return "", err
	}
	if kr.sphinxListener.dec.IsInSpeech() {
		kr.sphinxListener.inSpeech = true
		if !kr.sphinxListener.uttStarted {
			kr.sphinxListener.uttStarted = true
		}
	} else if kr.sphinxListener.uttStarted {
		// speech -> opusSilence transition, time to start new utterance
		kr.sphinxListener.dec.EndUtt()
		kr.sphinxListener.uttStarted = false
		hyp, _ := kr.sphinxListener.dec.Hypothesis()
		if len(hyp) > 0 {
			//restarting utt frees the memory containing hyp causing corruption since its based on a c library
			//probably a bug. safehyp is just forcing golang to copy the value of hyp and not the pointer.
			kr.pcmBuffer = nil
			safeHyp := (hyp + " ")[:len(hyp)]
			kr.sphinxListener.dec.StartUtt()
			return safeHyp, nil
		}
		if !kr.sphinxListener.dec.StartUtt() {
			zap.S().Info("Sphinx failed to start utt")
		}
	}
	kr.pcmBuffer = nil
	return "", nil
}

func resamplePCM(pcm []int16) ([]int16, error) {
	//should figure out a way to reuse the sampler will probably speed things up
	pcmBytes, err := int16SliceToByteSlice(pcm)
	if err != nil {
		return nil, err
	}
	var resampledPCMBytes bytes.Buffer
	resampledBytesWriter := io.Writer(&resampledPCMBytes)
	res, err := resample.New(resampledBytesWriter, float64(discordSampleRate), float64(sphinxSampleRate), 2, resample.I16, resample.HighQ)
	defer res.Close()
	if err != nil {
		return nil, err
	}
	_, err = res.Write(pcmBytes)
	if err != nil {
		return nil, err
	}
	resampledPCM, err := byteSliceToInt16Slice(resampledPCMBytes.Bytes())
	if err != nil {
		return nil, err
	}
	return resampledPCM, nil
}
