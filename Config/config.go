package Config

import (
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

//should be able to set config path value maybe as enviroment varible or command line option then default to this const
const path = "/home/chinz/go/src/DiscordVoiceRecognition/config.yml"

//might want to split this out into more fine grained structs
type Config struct {
	Discord struct {
		Token        string `yaml:"token"`
		Guild        string `yaml:"guild"`
		VoiceChannel string `yaml:"voicechannel"`
		TextChannel  string `yaml:"textchannel"`
	}
	Sphinx struct {
		HMM          string `yaml:"hmm"`
		Dict         string `yaml:"dict"`
		KeywordsFile string `yaml:"keywordsfile"`
		LogFile      string `yaml:"logfile"`
	}
	GoogleServices struct {
		CredentialsFile string `yaml:"credentialsfile"`
	}
	Rasa struct {
		Scheme   string `yaml:"scheme"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Project  string `yaml:"project"`
		Language string `yaml:"language"`
		Pipeline string `yaml:"pipeline"`
	}
	RemoteBot struct {
		Address string `yaml:"address"`
	}
	Log struct {
		Path string `yaml:"path"`
	}
}

func LoadConfig() Config {
	var config Config
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open config file located at %s", path)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Failed to stat config file located at %s", path)
	}
	fileSize := fileInfo.Size()
	buffer := make([]byte, fileSize)
	file.Read(buffer)
	err = yaml.Unmarshal(buffer, &config)
	if err != nil {
		log.Fatalf("Config data is most likely malformed at %s: %s", path, err)
	}
	return config
}
