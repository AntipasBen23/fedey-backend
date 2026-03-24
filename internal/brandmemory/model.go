package brandmemory

import "time"

type Profile struct {
	ID         string    `json:"id"`
	BrandName  string    `json:"brandName"`
	Tone       string    `json:"tone"`
	Audience   string    `json:"audience"`
	Pillars    []string  `json:"pillars"`
	Guardrails []string  `json:"guardrails"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type UpsertInput struct {
	BrandName  string   `json:"brandName"`
	Tone       string   `json:"tone"`
	Audience   string   `json:"audience"`
	Pillars    []string `json:"pillars"`
	Guardrails []string `json:"guardrails"`
}
