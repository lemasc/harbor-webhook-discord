package main

type HarborEvent struct {
	Type      string    `json:"type"`
	OccurAt   int64     `json:"occur_at"`
	Operator  string    `json:"operator"`
	EventData EventData `json:"event_data"`
}

type EventData struct {
	Resources  []Resource `json:"resources"`
	Repository Repository `json:"repository"`
}

type Resource struct {
	Digest      string `json:"digest"`
	Tag         string `json:"tag"`
	ResourceURL string `json:"resource_url"`
}

type Repository struct {
	DateCreated int64  `json:"date_created"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	FullName    string `json:"repo_full_name"`
	Type        string `json:"repo_type"`
}
