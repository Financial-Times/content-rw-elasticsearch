package concept

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	ThingURIPrefix         = "http://api.ft.com/things/"
	concordancesEndpoint   = "/concordances"
	concordancesQueryParam = "conceptId"
	tmeAuthority           = "http://api.ft.com/system/FT-TME"
	uppAuthority           = "http://api.ft.com/system/UPP"
)

type Concept struct {
	ID     string `json:"id"`
	APIURL string `json:"apiUrl,omitempty"`
}

type Identifier struct {
	IdentifierValue string `json:"identifierValue"`
	Authority       string `json:"authority"`
}

type Concordance struct {
	Concept    Concept    `json:"concept"`
	Identifier Identifier `json:"identifier"`
}

type ConcordancesResponse struct {
	Concordances []Concordance `json:"concordances"`
}

type Model struct {
	TmeIDs []string
}

type Reader interface {
	GetConcepts(tid string, ids []string) (map[string]Model, error)
}

type Client interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type ConcordanceAPIService struct {
	ConcordanceAPIBaseURL string
	Client                Client
}

func NewConcordanceAPIService(concordanceAPIBaseURL string, c Client) *ConcordanceAPIService {
	return &ConcordanceAPIService{ConcordanceAPIBaseURL: concordanceAPIBaseURL, Client: c}
}

func (c *ConcordanceAPIService) GetConcepts(tid string, ids []string) (map[string]Model, error) {
	req, err := http.NewRequest(http.MethodGet, c.ConcordanceAPIBaseURL+concordancesEndpoint, nil)
	if err != nil {
		return nil, err
	}

	queryParams := req.URL.Query()
	for _, id := range ids {
		queryParams.Add(concordancesQueryParam, id)
	}
	req.URL.RawQuery = queryParams.Encode()

	req.Header.Add("User-Agent", "UPP content-rw-elasticsearch")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Request-Id", tid)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calling Concordance API returned HTTP status %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var concordancesResp ConcordancesResponse
	if err = json.Unmarshal(body, &concordancesResp); err != nil {
		return nil, err
	}

	return TransformToConceptModel(concordancesResp), nil
}

func TransformToConceptModel(concordancesResp ConcordancesResponse) map[string]Model {
	conceptMap := make(map[string]Model)
	for _, c := range concordancesResp.Concordances {
		_, found := conceptMap[c.Concept.ID]
		if !found {
			conceptMap[c.Concept.ID] = Model{}
		}

		if c.Identifier.Authority == tmeAuthority {
			concept := conceptMap[c.Concept.ID]
			concept.TmeIDs = append(concept.TmeIDs, c.Identifier.IdentifierValue)
			conceptMap[c.Concept.ID] = concept
		}
		if c.Identifier.Authority == uppAuthority {
			_, found := conceptMap[ThingURIPrefix+c.Identifier.IdentifierValue]
			if !found {
				conceptMap[ThingURIPrefix+c.Identifier.IdentifierValue] = conceptMap[c.Concept.ID]
			}
		}
	}

	return conceptMap
}

func (c *ConcordanceAPIService) HealthCheck() (string, error) {
	req, err := http.NewRequest(http.MethodGet, c.ConcordanceAPIBaseURL+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "UPP content-rw-elasticsearch")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return "Concordance API is healthy", nil
}
