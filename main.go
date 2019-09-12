package main

import (
	"DiscordVoiceRecognition/Config"
	"DiscordVoiceRecognition/RasaNLU"
	"DiscordVoiceRecognition/VoiceRecognition"
	"encoding/json"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	//login and configuration setup
	config := Config.LoadConfig()

	logConfig := zap.NewDevelopmentConfig()
	logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logConfig.OutputPaths = []string{
		"stderr",
		config.Log.Path,
	}
	logger, err := logConfig.Build()
	if err != nil {
		log.Fatal(err)
	}
	zap.ReplaceGlobals(logger)
	//setting up enviroment varible containing google authentication credentials file there seems
	//to be no other way to do this
	err = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", config.GoogleServices.CredentialsFile)
	if err != nil {
		zap.S().Fatalf("Failed to add needed environment variable GOOGLE_APPLICATION_CREDENTIALS %s", err)
	}

	//train language model
	zap.S().Info("training language model")
	trainDataFile, err := os.Open("RasaTrainingData/traindata.json")
	defer trainDataFile.Close()
	if err != nil {
		zap.S().Fatal(err)
	}
	trainDataBytes, err := ioutil.ReadAll(trainDataFile)
	if err != nil {
		zap.S().Fatal(err)
	}

	trainData := RasaNLU.TrainData{}
	if err = json.Unmarshal([]byte(trainDataBytes), &trainData); err != nil {
		zap.S().Fatal(err)
	}
	if err = RasaNLU.Train(config.Rasa.Project, config.Rasa.Language, config.Rasa.Pipeline, trainData); err != nil {
		zap.S().Fatal(err)
	}

	//start voice recognition
	cvr := VoiceRecognition.CreateChannelVoiceRecognitionController()
	// Closes application on ctrl-c
	zap.S().Info("Setup Complete")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	//closing discord connection
	complete := cvr.Close()
	zap.S().Info("close sent. closing discord connection and cleaning up")
	<-complete
	zap.S().Info("finished")
	zap.S().Sync()
}
