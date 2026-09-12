package domain

// Preferences contains user choices, never provider credentials.
type Preferences struct {
	Provider string `json:"provider"`
}

func DefaultPreferences() Preferences { return Preferences{Provider: "openai"} }
