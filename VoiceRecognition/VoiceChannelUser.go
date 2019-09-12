package VoiceRecognition

type VoiceChannelUser struct {
	userId               string
	ssrc                 uint32
	keyPhraseRecognition *KeyPhraseRecognition
	silenceFrames        int
	speaking             bool
}

func createVoiceChannelUser(userId string, ssrc uint32, keywordSpokenNotify chan KeywordSpokenNotify, silenceFrames int) (*VoiceChannelUser, error) {
	keyPhraseRecognition, err := createKeyPhraseRecognition(keywordSpokenNotify)
	if err != nil {
		return nil, err
	}
	return &VoiceChannelUser{
		userId:               userId,
		ssrc:                 ssrc,
		keyPhraseRecognition: keyPhraseRecognition,
		silenceFrames:        silenceFrames,
	}, nil
}

type VoiceChannelUsers struct {
	byUserId map[string]*VoiceChannelUser
	bySSRC   map[uint32]*VoiceChannelUser
}

func createVoiceChannelUsers() *VoiceChannelUsers {
	return &VoiceChannelUsers{
		byUserId: make(map[string]*VoiceChannelUser),
		bySSRC:   make(map[uint32]*VoiceChannelUser),
	}
}

func (vcus *VoiceChannelUsers) add(userId string, ssrc uint32, keywordSpokenNotify chan KeywordSpokenNotify, silenceFrames int) error {
	voiceChannelUser, err := createVoiceChannelUser(userId, ssrc, keywordSpokenNotify, silenceFrames)
	if err != nil {
		return err
	}
	vcus.byUserId[userId] = voiceChannelUser
	vcus.bySSRC[ssrc] = voiceChannelUser
	return nil
}

func (vcus *VoiceChannelUsers) remove(userId string) {
	voiceChannelUser, exists := vcus.byUserId[userId]
	if !exists {
		return
	}
	close(voiceChannelUser.keyPhraseRecognition.VoiceInfoRecv)
	delete(vcus.bySSRC, voiceChannelUser.ssrc)
	delete(vcus.byUserId, userId)
}
