package RasaNLU

import (
	"DiscordVoiceRecognition/Config"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"net/url"
)

func Train(project string, language string, pipeline string, trainData TrainData) error {
	requestBodyBytes, err := generateTrainingDataRequestBody(language, pipeline, trainData)
	if err != nil {
		return err
	}
	requestBody := bytes.NewReader(requestBodyBytes)
	client := &http.Client{}
	req, err := http.NewRequest("POST", getUrl("train"), requestBody)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-yml")
	q := url.Values{}
	q.Add("project", project)
	req.URL.RawQuery = q.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		body := string(bodyBytes)
		return errors.New(fmt.Sprintf("Non 200 status:\nStatus: %s\nBody: %s", resp.Status, body))
	}
	return nil
}

func Parse(text string, project string) (*ParserResponse, error) {
	requestJson, err := json.Marshal(ParserRequest{Query: text, Project: project})
	if err != nil {
		return nil, err
	}
	requestBody := bytes.NewReader(requestJson)
	resp, err := http.Post(getUrl("parse"), "application/json", requestBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		body := string(bodyBytes)
		return nil, errors.New(fmt.Sprintf("Non 200 status:\nStatus: %s\nBody: %s", resp.Status, body))
	}
	parserResponse := &ParserResponse{}
	if err = json.NewDecoder(resp.Body).Decode(parserResponse); err != nil {
		return nil, err
	}
	return parserResponse, nil
}

func Status() (*StatusResponse, error) {
	resp, err := http.Get(getUrl("status"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		body := string(bodyBytes)
		return nil, errors.New(fmt.Sprintf("Non 200 status:\nStatus: %s\nBody: %s", resp.Status, body))
	}
	statusResponse := &StatusResponse{}
	if err = json.NewDecoder(resp.Body).Decode(statusResponse); err != nil {
		return nil, err
	}
	return statusResponse, nil
}

func getUrl(path string) string {
	config := Config.LoadConfig()
	baseURL := url.URL{
		Scheme: config.Rasa.Scheme,
		Host:   config.Rasa.Host + ":" + config.Rasa.Port,
		Path:   path,
	}
	return baseURL.String()
}

//they use a nested json within yaml so its not as simple as converting the struct to yaml or json
//have to generate each individually and then add in the json to the yaml
func generateTrainingDataRequestBody(language string, pipeline string, trainData TrainData) ([]byte, error) {
	trainDataBytes, err := json.Marshal(trainData)
	if err != nil {
		return nil, err
	}
	trainDataString := fmt.Sprintf("\ndata: {\"rasa_nlu_data\": %s }", string(trainDataBytes))
	trainRequest := TrainRequest{Language: language, Pipeline: pipeline}
	trainRequestyml, err := yaml.Marshal(trainRequest)
	if err != nil {
		return nil, err
	}
	trainDataRequestString := string(trainRequestyml) + trainDataString
	return []byte(trainDataRequestString), nil
}
