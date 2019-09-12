package VoiceRecognition

import (
	"DiscordVoiceRecognition/Config"
	"bytes"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
	"io/ioutil"
	"time"
)

type ChannelVoiceRecognitionController struct {
	session                  *discordgo.Session
	voiceConnection          *discordgo.VoiceConnection
	channelConnectedUsers    *VoiceChannelUsers
	commandNotify            chan string
	userSpeakingCommand      uint32
	commandProcessed         chan bool
	userDisconnect           chan string
	userConnect              chan userConnectInfo
	KeywordRecognitionNotify chan KeywordSpokenNotify
	commandRecognition       *CommandRecognition
	close                    chan chan bool
}

type KeywordSpokenNotify struct {
	ssrc      uint32
	keyPhrase string
}

type userConnectInfo struct {
	ssrc   uint32
	userId string
}

type VoiceInfo struct {
	packet   *discordgo.Packet
	speaking bool
}

func CreateChannelVoiceRecognitionController() ChannelVoiceRecognitionController {
	config := Config.LoadConfig()
	cvr := ChannelVoiceRecognitionController{
		channelConnectedUsers:    createVoiceChannelUsers(),
		userDisconnect:           make(chan string),
		userConnect:              make(chan userConnectInfo),
		KeywordRecognitionNotify: make(chan KeywordSpokenNotify),
		commandNotify:            make(chan string),
		close:                    make(chan chan bool),
	}
	zap.S().Info("Connecting to discord")
	//create discord bot
	discord, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		zap.S().Fatalf("Error creating Discord session: %s", err)
	}

	disconnectHandler(discord, cvr.userDisconnect)

	err = discord.Open()
	if err != nil {
		zap.S().Fatalf("Error opening Discord session: %s", err)
	}

	zap.S().Info("joining voice channel")
	//join voice channel
	voice, err := discord.ChannelVoiceJoin(config.Discord.Guild, config.Discord.VoiceChannel, false, false)
	if err != nil {
		zap.S().Fatalf("Failed to join voice channel: %s", err)
	}
	connectHandler(voice, cvr.userConnect)
	//play join sound
	startupWav, err := ioutil.ReadFile("VoiceRecognition/Sounds/startup.wav")
	if err != nil {
		zap.S().Fatal(err)
	}

	err = playWaveAudio(startupWav, voice)
	if err != nil {
		zap.S().Fatal(err)
	}

	//this opus silence is a full silence packet its not the kind discord uses to detect a user speaking or not speaking
	//this is sent to force the discord connection to start sending voice data
	cvr.session = discord
	cvr.voiceConnection = voice
	go cvr.Start()
	return cvr
}

func (cvr *ChannelVoiceRecognitionController) Start() {
	unknownUsersSilencePackets := make(map[uint32]int)
	pulseStop := make(chan bool)
	for {
		select {
		case userJoined := <-cvr.userConnect:
			user, err := cvr.session.User(userJoined.userId)
			if err != nil || user.Bot {
				if user.Bot{
					zap.S().Debug(
						"User was not added because bot",
						zap.String("userid", userJoined.userId),
						zap.String("operation", "User Joined"),
						)
				}
				continue
			}
			silenceFrames := 0
			if _, exists := cvr.channelConnectedUsers.bySSRC[userJoined.ssrc]; exists {
				continue
			}
			if unknownUserSilencePackets, exists := unknownUsersSilencePackets[userJoined.ssrc]; exists {
				silenceFrames = unknownUserSilencePackets
			}
			if err := cvr.channelConnectedUsers.add(userJoined.userId, userJoined.ssrc, cvr.KeywordRecognitionNotify, silenceFrames); err != nil {
				zap.S().Info(
					"Failed to add user to connected users",
					zap.String("userid", userJoined.userId),
					zap.String("operation", "User Joined"),
					zap.String("err", err.Error()),
					)
			}
			zap.S().Info(
				"User connected",
				zap.String("userid", userJoined.userId),
				zap.String("operation", "User Joined"),
				)
		case userIdLeft := <-cvr.userDisconnect:
			cvr.channelConnectedUsers.remove(userIdLeft)
			zap.S().Info(
				"User disconnected",
				zap.String("userid", userIdLeft),
				zap.String("operation", "User Disconnect"),
			)

		case opusPacket := <-cvr.voiceConnection.OpusRecv:
			zap.S().Debug("sorting voice packet start")
			if _, exists := cvr.channelConnectedUsers.bySSRC[opusPacket.SSRC]; !exists {
				//this is a hack to get around the fact that silence packets are sent before the user joined event triggers

				//this dosnt take into account bot users
				unknownUsersSilencePackets[opusPacket.SSRC]++
				continue
			}
			voiceInfo := cvr.buildVoiceInfo(opusPacket)
			if voiceInfo == nil {
				continue
			}

			if cvr.userSpeakingCommand != opusPacket.SSRC {
				cvr.channelConnectedUsers.bySSRC[opusPacket.SSRC].keyPhraseRecognition.VoiceInfoRecv <- voiceInfo
				zap.S().Debug("sorting voice packet end key phrase recognition")
				continue
			}
			zap.S().Debug("sorting voice packet end command recognition start")
			//non blocking since receiving function might complete before cleanup
			//added significant buffer to voiceInfoRecv so packets getting sent to fast aren't ignored
			select{
				case cvr.commandRecognition.VoiceInfoRecv <- voiceInfo:
			}
			zap.S().Debug("sorting voice packet end command recognition end")

		case command := <-cvr.commandNotify:
			zap.S().Infof("user %s said command \"%s\"", cvr.channelConnectedUsers.bySSRC[cvr.userSpeakingCommand].userId, command)
			pulseStop <- true
			cvr.commandProcessed = commandProcessing(cvr.channelConnectedUsers.bySSRC[cvr.userSpeakingCommand].userId, command, cvr.voiceConnection)

		case <-cvr.commandProcessed:
			zap.S().Infof("Completed listing of command and processing for user %s", cvr.channelConnectedUsers.bySSRC[cvr.userSpeakingCommand].userId)
			cvr.commandRecognition = nil
			cvr.userSpeakingCommand = 0

		case keywordNotify := <-cvr.KeywordRecognitionNotify:
			if _, exists := cvr.channelConnectedUsers.bySSRC[keywordNotify.ssrc]; !exists {
				continue
			}
			zap.S().Infof("user %s said keyword %s", cvr.channelConnectedUsers.bySSRC[keywordNotify.ssrc].userId, keywordNotify.keyPhrase)
			if cvr.userSpeakingCommand != 0 {
				zap.S().Infof("user %s can't use command recognition already in use by user %s", cvr.channelConnectedUsers.bySSRC[keywordNotify.ssrc].userId, cvr.channelConnectedUsers.bySSRC[cvr.userSpeakingCommand].userId)
				continue
			}
			cvr.userSpeakingCommand = keywordNotify.ssrc
			startupWav, err := ioutil.ReadFile("VoiceRecognition/Sounds/Listening.wav")
			if err != nil {
				zap.S().Info(err)
			}

			err = playWaveAudio(startupWav, cvr.voiceConnection)
			if err != nil {
				zap.S().Info(err)
			}
			pulseBot(pulseStop, cvr.voiceConnection)
			cvr.commandRecognition = createCommandRecognition(cvr.commandNotify)

		case complete := <-cvr.close:
			for _, connectedUser := range cvr.channelConnectedUsers.byUserId {
				close(connectedUser.keyPhraseRecognition.VoiceInfoRecv)

			}
			if cvr.commandRecognition != nil {
				cvr.commandRecognition.close <- true
			}
			cvr.voiceConnection.Close()
			if err := cvr.session.Close(); err != nil {
				zap.S().Warn(err)
			}
			complete <- true
			return
		}
	}
}

func pulseBot(pulseStop <-chan bool, voiceConnection *discordgo.VoiceConnection) {
	go func() {
		realSilenceFrame := []byte{
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		for {
			ticker := time.NewTicker(1500 * time.Millisecond)
			voiceConnection.Speaking(true)
			voiceConnection.OpusSend <- realSilenceFrame
			voiceConnection.Speaking(false)
			select {
			case <-ticker.C:
				voiceConnection.Speaking(true)
				voiceConnection.OpusSend <- realSilenceFrame
				voiceConnection.Speaking(false)
			case <-pulseStop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (cvr *ChannelVoiceRecognitionController) Close() chan bool {
	complete := make(chan bool)
	cvr.close <- complete
	return complete
}

func disconnectHandler(session *discordgo.Session, userDisconnect chan<- string) {
	session.AddHandler(func(session *discordgo.Session, state *discordgo.VoiceStateUpdate) {
		if len(state.ChannelID) == 0 {
			userDisconnect <- state.UserID
		}
	})
}

func connectHandler(connection *discordgo.VoiceConnection, userConnect chan<- userConnectInfo) {
	connection.AddHandler(func(vc *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		userConnect <- userConnectInfo{
			ssrc:   uint32(vs.SSRC),
			userId: vs.UserID,
		}
	})
}

func (cvr *ChannelVoiceRecognitionController) buildVoiceInfo(packet *discordgo.Packet) *VoiceInfo {
	channelConnectedUser := cvr.channelConnectedUsers.bySSRC[packet.SSRC]
	if !bytes.Equal(packet.Opus, opusSilence) {
		return &VoiceInfo{packet: packet, speaking: true}
	}
	channelConnectedUser.silenceFrames++
	if channelConnectedUser.silenceFrames == 6 {
		return &VoiceInfo{packet: packet, speaking: false}
	}
	if channelConnectedUser.silenceFrames >= 10 {
		channelConnectedUser.silenceFrames = 0
	}
	return nil
}
