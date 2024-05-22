package schema

type IndexModel struct {
	Type                       *string  `json:"type"`
	UID                        *string  `json:"uid"`
	LastMetadataPublish        *string  `json:"last_metadata_publish"`
	IndexDate                  *string  `json:"index_date"`
	MarkDeleted                bool     `json:"mark_deleted"`
	StoryID                    *int32   `json:"story_id"`
	LeadHeadline               *string  `json:"lead_headline"`
	Byline                     *string  `json:"byline"`
	Body                       *string  `json:"body"`
	Scoop                      bool     `json:"scoop"`
	CanBeSyndicated            *string  `json:"canBeSyndicated"`
	CanBeDistributed           *string  `json:"canBeDistributed"`
	URL                        *string  `json:"url"`
	InitialPublish             *string  `json:"initial_publish"`
	LastPublish                *string  `json:"last_publish"`
	ContentType                *string  `json:"content_type"`
	ProviderName               *string  `json:"provider_name"`
	LengthMillis               int32    `json:"length_millis"`
	ShortDescription           *string  `json:"short_description"`
	ThumbnailURL               *string  `json:"thumbnail_url"`
	SectionLink                *string  `json:"section_link"`
	SecondaryImageID           *string  `json:"secondary_image_id"`
	ContributorRights          *string  `json:"contributor_rights"`
	SourceCode                 *string  `json:"source_code"`
	StorymodelID               *int32   `json:"storymodel_id"`
	ModelAPIURL                *string  `json:"model_api_url"`
	ModelMasterSource          *string  `json:"model_master_source"`
	ModelMasterID              *string  `json:"model_master_id"`
	ModelExcerpt               *string  `json:"model_excerpt"`
	ModelResourceURI           *string  `json:"model_resource_uri"`
	CmrPrimarysection          *string  `json:"cmr_primarysection"`
	CmrPrimarytheme            *string  `json:"cmr_primarytheme"`
	CmrMediatype               *string  `json:"cmr_mediatype"`
	CmrMetadataupdatetime      *string  `json:"cmr_metadataupdatetime"`
	CmrPrimarysectionID        *string  `json:"cmr_primarysection_id"`
	CmrPrimarythemeID          *string  `json:"cmr_primarytheme_id"`
	CmrMediatypeID             *string  `json:"cmr_mediatype_id"`
	CmrBrands                  []string `json:"cmr_brands"`
	CmrBrandsIds               []string `json:"cmr_brands_ids"`
	CmrSpecialreports          []string `json:"cmr_specialreports"`
	CmrSpecialreportsIds       []string `json:"cmr_specialreports_ids"`
	CmrSections                []string `json:"cmr_sections"`
	CmrSectionsIds             []string `json:"cmr_sections_ids"`
	CmrSubjects                []string `json:"cmr_subjects"`
	CmrSubjectsIds             []string `json:"cmr_subjects_ids"`
	CmrTopics                  []string `json:"cmr_topics"`
	CmrTopicsIds               []string `json:"cmr_topics_ids"`
	CmrPeople                  []string `json:"cmr_people"`
	CmrPeopleIds               []string `json:"cmr_people_ids"`
	CmrRegions                 []string `json:"cmr_regions"`
	CmrRegionsIds              []string `json:"cmr_regions_ids"`
	CmrIcb                     []string `json:"cmr_icb"`
	CmrIcbIds                  []string `json:"cmr_icb_ids"`
	CmrIptc                    []string `json:"cmr_iptc"`
	CmrIptcIds                 []string `json:"cmr_iptc_ids"`
	CmrAuthorsIds              []string `json:"cmr_authors_ids"`
	CmrAuthors                 []string `json:"cmr_authors"`
	CmrCompanynames            []string `json:"cmr_companynames"`
	CmrCompanynamesIds         []string `json:"cmr_companynames_ids"`
	CmrOrgnames                []string `json:"cmr_orgnames"`
	CmrOrgnamesIds             []string `json:"cmr_orgnames_ids"`
	BestStory                  bool     `json:"bestStory"`
	InternalContentType        *string  `json:"internalContentType"`
	Category                   *string  `json:"category"`
	LookupFailure              bool     `json:"lookupFailure"`
	Format                     *string  `json:"format"`
	CmrGenres                  []string `json:"cmr_genre"`
	CmrGenreIds                []string `json:"cmr_genre_id"`
	Region                     *string  `json:"region"`
	Topics                     []string `json:"topics"`
	DisplayCodes               []string `json:"displayCodes"`
	DisplayCodeNames           []string `json:"displayCodeNames"`
	NaicsNames                 []string `json:"naicsNames"`
	EditorsTags                []string `json:"editorsTags"`
	CountryCodes               []string `json:"countryCodes"`
	CountryNames               []string `json:"countryNames"`
	Subjects                   []string `json:"subjects"`
	CompanyNamesAuto           []string `json:"companyNamesAuto"`
	OrganisationNamesAuto      []string `json:"organisationNamesAuto"`
	CompanyNamesEditorial      []string `json:"companyNamesEditorial"`
	CompanyTickerCodeAuto      []string `json:"companyTickerCodeAuto"`
	CompanyTickerCodeEditorial []string `json:"companyTickerCodeEditorial"`
	ArticleTypes               []string `json:"articleTypes"`
	ArticleBrands              []string `json:"articleBrands"`
	PublishReference           string   `json:"publishReference"`
}

type EnrichedContent struct {
	UUID            string          `json:"uuid"`
	Content         Content         `json:"content"`
	Metadata        Annotations     `json:"metadata"`
	InternalContent InternalContent `json:"internalContent"`
	ContentURI      string          `json:"contentUri"`
	LastModified    string          `json:"lastModified"`
	Deleted         bool            `json:"deleted"`
}

type Content struct {
	UUID               string       `json:"uuid"`
	Title              string       `json:"title"`
	Body               string       `json:"body"`
	BodyXML            string       `json:"bodyXML,omitempty"`
	Identifiers        []identifier `json:"identifiers"`
	PublishedDate      string       `json:"publishedDate"`
	LastModified       string       `json:"lastModified"`
	FirstPublishedDate string       `json:"firstPublishedDate"`
	MarkedDeleted      bool         `json:"marked_deleted"`
	Byline             string       `json:"byline"`
	Standfirst         string       `json:"standfirst"`
	Description        string       `json:"description"`
	MainImage          string       `json:"mainImage"`
	PublishReference   string       `json:"publishReference"`
	Type               string       `json:"type"`
	DataSources        []dataSource `json:"dataSource"`
	Scoop              bool         `json:"scoop"`
	CanBeSyndicated    *string      `json:"canBeSyndicated"`
	CanBeDistributed   *string      `json:"canBeDistributed"`
	Publication        []string     `json:"publication"`
}

type dataSource struct {
	Duration  int32  `json:"duration"`
	MediaType string `json:"mediaType"`
}

type identifier struct {
	Authority       string `json:"authority"`
	IdentifierValue string `json:"identifierValue"`
}

type Annotations []Annotation

// Annotation is the main struct used to create and return annotations
type Annotation struct {
	Thing Thing `json:"thing,omitempty"`
}

// Thing represents a concept being linked to
type Thing struct {
	ID        string   `json:"id,omitempty"`
	PrefLabel string   `json:"prefLabel,omitempty"`
	Types     []string `json:"types,omitempty"`
	Predicate string   `json:"predicate,omitempty"`
}

type ContentType struct {
	Collection string
	Format     string
	Category   string
}

type InternalContent struct {
	MainImage ImageSet   `json:"mainImage"`
	Embeds    []ImageSet `json:"embeds"`
}

type ImageSet struct {
	ID      string   `json:"id"`
	Members []Member `json:"members"`
}

type Member struct {
	APIURL string `json:"apiUrl"`
}
