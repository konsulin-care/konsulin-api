package requests

type Questionnaire struct {
	ResourceType      string              `json:"resourceType"`
	ID                string              `json:"id,omitempty"`
	Meta              Meta                `json:"meta,omitempty"`
	ImplicitRules     string              `json:"implicitRules,omitempty"`
	Language          string              `json:"language,omitempty"`
	Text              Narrative           `json:"text,omitempty"`
	Extension         []Extension         `json:"extension,omitempty"`
	ModifierExtension []Extension         `json:"modifierExtension,omitempty"`
	Url               string              `json:"url,omitempty"`
	Identifier        []Identifier        `json:"identifier,omitempty"`
	Version           string              `json:"version,omitempty"`
	Name              string              `json:"name,omitempty"`
	Title             string              `json:"title,omitempty"`
	DerivedFrom       []string            `json:"derivedFrom,omitempty"`
	Status            string              `json:"status"`
	Experimental      bool                `json:"experimental,omitempty"`
	SubjectType       []string            `json:"subjectType,omitempty"`
	Date              string              `json:"date,omitempty"`
	Publisher         string              `json:"publisher,omitempty"`
	Contact           []ContactDetail     `json:"contact,omitempty"`
	Description       string              `json:"description,omitempty"`
	UseContext        []UsageContext      `json:"useContext,omitempty"`
	Jurisdiction      []CodeableConcept   `json:"jurisdiction,omitempty"`
	Purpose           string              `json:"purpose,omitempty"`
	Copyright         string              `json:"copyright,omitempty"`
	ApprovalDate      string              `json:"approvalDate,omitempty"`
	LastReviewDate    string              `json:"lastReviewDate,omitempty"`
	EffectivePeriod   Period              `json:"effectivePeriod,omitempty"`
	Code              []CodeableConcept   `json:"code,omitempty"`
	Item              []QuestionnaireItem `json:"item,omitempty"`
}

type QuestionnaireItem struct {
	LinkID         string                          `json:"linkId"`
	Definition     string                          `json:"definition,omitempty"`
	Code           []CodeableConcept               `json:"code,omitempty"`
	Prefix         string                          `json:"prefix,omitempty"`
	Text           string                          `json:"text,omitempty"`
	Type           string                          `json:"type"`
	EnableWhen     []QuestionnaireItemEnableWhen   `json:"enableWhen,omitempty"`
	EnableBehavior string                          `json:"enableBehavior,omitempty"`
	Required       bool                            `json:"required,omitempty"`
	Repeats        bool                            `json:"repeats,omitempty"`
	ReadOnly       bool                            `json:"readOnly,omitempty"`
	MaxLength      int                             `json:"maxLength,omitempty"`
	AnswerValueSet string                          `json:"answerValueSet,omitempty"`
	AnswerOption   []QuestionnaireItemAnswerOption `json:"answerOption,omitempty"`
	Initial        []QuestionnaireItemInitial      `json:"initial,omitempty"`
	Item           []QuestionnaireItem             `json:"item,omitempty"`
}

type QuestionnaireItemEnableWhen struct {
	Question        string    `json:"question"`
	Operator        string    `json:"operator"`
	AnswerBoolean   bool      `json:"answerBoolean,omitempty"`
	AnswerDecimal   float64   `json:"answerDecimal,omitempty"`
	AnswerInteger   int       `json:"answerInteger,omitempty"`
	AnswerDate      string    `json:"answerDate,omitempty"`
	AnswerDateTime  string    `json:"answerDateTime,omitempty"`
	AnswerTime      string    `json:"answerTime,omitempty"`
	AnswerString    string    `json:"answerString,omitempty"`
	AnswerCoding    Coding    `json:"answerCoding,omitempty"`
	AnswerQuantity  Quantity  `json:"answerQuantity,omitempty"`
	AnswerReference Reference `json:"answerReference,omitempty"`
}

type QuestionnaireItemAnswerOption struct {
	ValueInteger    int       `json:"valueInteger,omitempty"`
	ValueDate       string    `json:"valueDate,omitempty"`
	ValueTime       string    `json:"valueTime,omitempty"`
	ValueString     string    `json:"valueString,omitempty"`
	ValueCoding     Coding    `json:"valueCoding,omitempty"`
	ValueReference  Reference `json:"valueReference,omitempty"`
	InitialSelected bool      `json:"initialSelected,omitempty"`
}

type QuestionnaireItemInitial struct {
	ValueBoolean    bool       `json:"valueBoolean,omitempty"`
	ValueDecimal    float64    `json:"valueDecimal,omitempty"`
	ValueInteger    int        `json:"valueInteger,omitempty"`
	ValueDate       string     `json:"valueDate,omitempty"`
	ValueDateTime   string     `json:"valueDateTime,omitempty"`
	ValueTime       string     `json:"valueTime,omitempty"`
	ValueString     string     `json:"valueString,omitempty"`
	ValueUri        string     `json:"valueUri,omitempty"`
	ValueAttachment Attachment `json:"valueAttachment,omitempty"`
	ValueCoding     Coding     `json:"valueCoding,omitempty"`
	ValueQuantity   Quantity   `json:"valueQuantity,omitempty"`
	ValueReference  Reference  `json:"valueReference,omitempty"`
}
