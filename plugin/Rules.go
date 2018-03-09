package plugin

type Rules struct {
	MinInstances  int  `json:"min_instances"`
	MaxInstances  int  `json:"max_instances"`
	Enabled       bool `json:"enabled"`
	Relationships struct {
		Rules []struct {
			GUID         string `json:"guid"`
			Type         string `json:"type"`
			Enabled      bool   `json:"enabled"`
			SubType      string `json:"sub_type"`
			MinThreshold int    `json:"min_threshold"`
			MaxThreshold int    `json:"max_threshold"`
		} `json:"rules"`
	} `json:"relationships"`
}

func (r *Rules) clean() error {
	for index, _ := range r.Relationships.Rules {
		r.Relationships.Rules[index].GUID = ""
	}

	return nil
}
