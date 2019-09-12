package RasaNLU

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestStatus(t *testing.T) {
	statusReponse, err := Status()
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+v", statusReponse)
}

func TestParse(t *testing.T) {
	parseResponse, err := Parse("hello", "airhorn")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%+v", parseResponse)
}

func TestTrain(t *testing.T) {
	data := `
	{
    "entity_synonyms": [
    {
      "value": "air horn",
      "synonyms": ["airhorn"]
    },
    {
      "value": "fog horn",
      "synonyms": ["foghorn"]
    }
    ],
    "common_examples": [
    {
      "text": "play air horn",
      "intent": "playhorn",
      "entities": [
      {
        "start": 5,
        "end": 8,
        "value": "air",
        "entity": "horntype"
      }
      ]
    },
    {
      "text": "play fog horn",
      "intent": "playhorn",
      "entities": [
      {
        "start": 5,
        "end": 8,
        "value": "fog",
        "entity": "horntype"
      }
      ]
    },
    {
      "text": "play air raid siren",
      "intent": "playhorn",
      "entities": [
      {
        "start": 5,
        "end": 19,
        "value": "air raid siren",
        "entity": "horntype"
      }
      ]
    },
    {
      "text": "play airhorn",
      "intent": "playhorn",
      "entities": [
      {
        "start": 5,
        "end": 12,
        "value": "air horn",
        "entity": "horntype"
      }
      ]
    },
    {
      "text": "hello",
      "intent": "greet",
      "entities": []
    },
    {
      "text": "hello there",
      "intent": "greet",
      "entities": []
    },
    {
      "text": "yeet",
      "intent": "greet",
      "entities": []
    }
    ]
	}
	`
	trainData := TrainData{}
	err := json.Unmarshal([]byte(data), &trainData)
	if err != nil {
		t.Error(err)
		return
	}
	err = Train("airhorn", "en", "spacy_sklearn", trainData)
	if err != nil {
		t.Error(err)
		return
	}
}
