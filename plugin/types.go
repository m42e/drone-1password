package plugin

type fullItem struct {
	ID         string        `json:"id"`
	Title      string        `json:"title"`
	Fields     []itemField   `json:"fields"`
	Sections   []itemSection `json:"sections"`
	NotesPlain string        `json:"notesPlain"`
}

type itemField struct {
	ID      string            `json:"id"`
	Label   string            `json:"label"`
	Value   string            `json:"value"`
	Purpose string            `json:"purpose"`
	Type    string            `json:"type"`
	Section *itemFieldSection `json:"section"`
}

type itemFieldSection struct {
	ID string `json:"id"`
}

type itemSection struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type vaultSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type itemSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
