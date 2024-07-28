package responses

type OperationOutcome struct {
	ResourceType string  `json:"resourceType"`
	Issue        []Issue `json:"issue"`
}

type Issue struct {
	Severity    string `json:"severity"`
	Diagnostics string `json:"diagnostics"`
}
