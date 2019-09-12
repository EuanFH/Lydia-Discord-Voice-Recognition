package RasaNLU

//parser
type ParserRequest struct {
	Query   string `json:"q"`
	Project string `json:"project,omitempty"`
}

type ParserResponse struct {
	Intent        Intent   `json:"intent"`
	Entities      []Entity `json:"entities"`
	IntentRanking []Intent `json:"intent_ranking"`
	Text          string   `json:"text"`
	Project       string   `json:"project"`
	Model         string   `json:"model"`
}

type Entity struct {
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Value      string  `json:"value"`
	Entity     string  `json:"entity"`
	Confidence float64 `json:"confidence"`
	Extractor  string  `json:"extractor"`
}

type Intent struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

//train
type TrainRequest struct {
	Language string `yaml:"language"`
	Pipeline string `yaml:"pipeline"`
}

type TrainData struct {
	EntitySynonyms []EntitySynonym `json:"entity_synonyms"`
	CommonExamples []Example       `json:"common_examples"`
}

type EntitySynonym struct {
	Value    string   `json:"value"`
	Synonyms []string `json:"synonyms"`
}

type Example struct {
	Text     string `json:"text"`
	Intent   string `json:"intent"`
	Entities []struct {
		Start  int    `json:"start"`
		End    int    `json:"end"`
		Value  string `json:"value"`
		Entity string `json:"entity"`
	} `json:"entities"`
}

//status
type StatusResponse struct {
	AvailableProjects map[string]struct {
		Status          string   `json:"status"`
		AvailableModels []string `json:"available_models"`
	} `json:"available_projects"`
}
