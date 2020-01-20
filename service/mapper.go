package service

import (
	"encoding/base64"
	"strings"
	"time"

	"fmt"

	"github.com/Financial-Times/content-rw-elasticsearch/v2/content"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/service/concept"
	"github.com/Financial-Times/content-rw-elasticsearch/v2/service/utils"
	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/uuid-utils-go"
	"github.com/pkg/errors"
)

const (
	isPrimaryClassifiedBy  = "http://www.ft.com/ontology/classification/isPrimarilyClassifiedBy"
	isClassifiedBy         = "http://www.ft.com/ontology/classification/isClassifiedBy"
	implicitlyClassifiedBy = "http://www.ft.com/ontology/implicitlyClassifiedBy"
	about                  = "http://www.ft.com/ontology/annotation/about"
	implicitlyAbout        = "http://www.ft.com/ontology/implicitlyAbout"
	mentions               = "http://www.ft.com/ontology/annotation/mentions"
	majorMentions          = "http://www.ft.com/ontology/annotation/majorMentions"
	hasDisplayTag          = "http://www.ft.com/ontology/hasDisplayTag"
	hasAuthor              = "http://www.ft.com/ontology/annotation/hasAuthor"
	hasContributor         = "http://www.ft.com/ontology/hasContributor"
	webURLPrefix           = "https://www.ft.com/content/"
	apiURLPrefix           = "/content/"
	imageServiceURL        = "https://www.ft.com/__origami/service/image/v2/images/raw/http%3A%2F%2Fprod-upp-image-read.ft.com%2F[image_uuid]?source=search&fit=scale-down&width=167"
	imagePlaceholder       = "[image_uuid]"

	tmeOrganisations = "ON"
	tmePeople        = "PN"
	tmeAuthors       = "Authors"
	tmeTopics        = "Topics"
	tmeRegions       = "GL"

	ArticleType = "article"
	VideoType   = "video"
	BlogType    = "blog"
	AudioType   = "audio"

	video_prefix = "video"
)

var noAnnotationErr = errors.New("No annotation to be processed")

var ContentTypeMap = map[string]content.ContentType{
	ArticleType: {
		Collection: "FTCom",
		Format:     "Articles",
		Category:   "article",
	},
	BlogType: {
		Collection: "FTBlogs",
		Format:     "Blogs",
		Category:   "blogPost",
	},
	VideoType: {
		Collection: "FTVideos",
		Format:     "Videos",
		Category:   "video",
	},
	AudioType: {
		Collection: "FTAudios",
		Format:     "Audios",
		Category:   "audio",
	},
}

func (handler *MessageHandler) ToIndexModel(enrichedContent content.EnrichedContent, contentType string, tid string) content.IndexModel {
	model := content.IndexModel{}

	baseApiUrl := handler.baseApiUrl
	if strings.HasPrefix(handler.baseApiUrl, "http://") {
		baseApiUrl = strings.Replace(handler.baseApiUrl, "http", "https", 1)
	}
	populateContentRelatedFields(&model, enrichedContent, contentType, tid, baseApiUrl)

	annotations, concepts, err := handler.prepareAnnotationsWithConcepts(&enrichedContent, tid)
	if err != nil {
		if err == noAnnotationErr {
			logger.WithTransactionID(tid).Warn(err.Error())
		} else {
			logger.WithError(err).WithTransactionID(tid).Error(err)
		}
		return model
	}

	for _, annotation := range annotations {
		canonicalID := strings.TrimPrefix(annotation.ID, concept.ThingURIPrefix)
		concept, found := concepts[annotation.ID]
		if !found {
			logger.WithTransactionID(tid).WithUUID(enrichedContent.UUID).Warnf("No concordance found for %v", canonicalID)
			continue
		}
		annIDs := []string{canonicalID}
		if concept.TmeIDs != nil {
			annIDs = append(annIDs, concept.TmeIDs...)
		} else {
			logger.WithTransactionID(tid).WithUUID(enrichedContent.Content.UUID).Warnf("TME id missing for concept with id %s, using only canonical id", canonicalID)
		}

		populateAnnotationRelatedFields(annotation, &model, annIDs, canonicalID)
	}
	return model
}

func populateAnnotationRelatedFields(annotation content.Thing, model *content.IndexModel, annIDs []string, canonicalID string) {
	handleSectionMapping(annotation, model, annIDs)
	for _, taxonomy := range annotation.Types {
		switch taxonomy {
		case "http://www.ft.com/ontology/organisation/Organisation":
			model.CmrOrgnames = appendIfNotExists(model.CmrOrgnames, annotation.PrefLabel)
			model.CmrOrgnamesIds = prepareElasticField(model.CmrOrgnamesIds, annIDs)
			if annotation.Predicate == about {
				setPrimaryTheme(model, annotation.PrefLabel, getCmrIDWithFallback(tmeOrganisations, annIDs))
			}
		case "http://www.ft.com/ontology/person/Person":
			_, personFound := getCmrID(tmePeople, annIDs)
			authorCmrID, authorFound := getCmrID(tmeAuthors, annIDs)
			// if it's only author, skip adding to people
			if personFound || !authorFound {
				model.CmrPeople = appendIfNotExists(model.CmrPeople, annotation.PrefLabel)
				model.CmrPeopleIds = prepareElasticField(model.CmrPeopleIds, annIDs)
			}
			if annotation.Predicate == hasAuthor || annotation.Predicate == hasContributor {
				if authorFound {
					model.CmrAuthors = appendIfNotExists(model.CmrAuthors, annotation.PrefLabel)
					model.CmrAuthorsIds = appendIfNotExists(model.CmrAuthorsIds, authorCmrID)
					model.CmrAuthorsIds = appendIfNotExists(model.CmrAuthorsIds, canonicalID)
				}
			}
			if annotation.Predicate == about {
				setPrimaryTheme(model, annotation.PrefLabel, getCmrIDWithFallback(tmePeople, annIDs))
			}
		case "http://www.ft.com/ontology/company/Company":
			model.CmrCompanynames = appendIfNotExists(model.CmrCompanynames, annotation.PrefLabel)
			model.CmrCompanynamesIds = prepareElasticField(model.CmrCompanynamesIds, annIDs)
		case "http://www.ft.com/ontology/product/Brand":
			model.CmrBrands = appendIfNotExists(model.CmrBrands, annotation.PrefLabel)
			model.CmrBrandsIds = prepareElasticField(model.CmrBrandsIds, annIDs)
		case "http://www.ft.com/ontology/Topic":
			model.CmrTopics = appendIfNotExists(model.CmrTopics, annotation.PrefLabel)
			model.CmrTopicsIds = prepareElasticField(model.CmrTopicsIds, annIDs)
			if annotation.Predicate == about {
				setPrimaryTheme(model, annotation.PrefLabel, getCmrIDWithFallback(tmeTopics, annIDs))
			}
		case "http://www.ft.com/ontology/Location":
			model.CmrRegions = appendIfNotExists(model.CmrRegions, annotation.PrefLabel)
			model.CmrRegionsIds = prepareElasticField(model.CmrRegionsIds, annIDs)
			if annotation.Predicate == about {
				setPrimaryTheme(model, annotation.PrefLabel, getCmrIDWithFallback(tmeRegions, annIDs))
			}
		case "http://www.ft.com/ontology/Genre":
			model.CmrGenres = appendIfNotExists(model.CmrGenres, annotation.PrefLabel)
			model.CmrGenreIds = prepareElasticField(model.CmrGenreIds, annIDs)
		}
	}
}

func (handler *MessageHandler) prepareAnnotationsWithConcepts(enrichedContent *content.EnrichedContent, tid string) ([]content.Thing, map[string]concept.ConceptModel, error) {
	var ids []string
	var anns []content.Thing
	for _, a := range enrichedContent.Metadata {
		if a.Thing.Predicate == mentions || a.Thing.Predicate == hasDisplayTag {
			//ignore these annotations
			continue
		}
		ids = append(ids, a.Thing.ID)
		anns = append(anns, a.Thing)
	}

	if len(ids) == 0 {
		return nil, nil, noAnnotationErr
	}

	concepts, err := handler.ConceptGetter.GetConcepts(tid, ids)
	return anns, concepts, err
}

func populateContentRelatedFields(model *content.IndexModel, enrichedContent content.EnrichedContent, contentType string, tid string, baseApiUrl string) {
	model.IndexDate = new(string)
	*model.IndexDate = time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
	model.ContentType = new(string)
	*model.ContentType = contentType
	model.InternalContentType = new(string)
	*model.InternalContentType = contentType
	model.Category = new(string)
	*model.Category = ContentTypeMap[contentType].Category
	model.Format = new(string)
	*model.Format = ContentTypeMap[contentType].Format
	model.UID = &(enrichedContent.Content.UUID)
	model.LeadHeadline = new(string)
	*model.LeadHeadline = utils.TransformText(enrichedContent.Content.Title,
		utils.HtmlEntityTransformer,
		utils.TagsRemover,
		utils.OuterSpaceTrimmer,
		utils.DuplicateWhiteSpaceRemover)
	model.Byline = new(string)
	*model.Byline = utils.TransformText(enrichedContent.Content.Byline,
		utils.HtmlEntityTransformer,
		utils.TagsRemover,
		utils.OuterSpaceTrimmer,
		utils.DuplicateWhiteSpaceRemover)
	if enrichedContent.Content.PublishedDate != "" {
		model.LastPublish = &(enrichedContent.Content.PublishedDate)
	}
	if enrichedContent.Content.FirstPublishedDate != "" {
		model.InitialPublish = &(enrichedContent.Content.FirstPublishedDate)
	}
	model.Body = new(string)
	if enrichedContent.Content.Body != "" {
		*model.Body = utils.TransformText(enrichedContent.Content.Body,
			utils.InteractiveGraphicsMarkupTagRemover,
			utils.PullTagTransformer,
			utils.HtmlEntityTransformer,
			utils.ScriptTagRemover,
			utils.TagsRemover,
			utils.OuterSpaceTrimmer,
			utils.Embed1Replacer,
			utils.SquaredCaptionReplacer,
			utils.DuplicateWhiteSpaceRemover)
	} else {
		*model.Body = enrichedContent.Content.Description
	}

	model.ShortDescription = new(string)
	*model.ShortDescription = enrichedContent.Content.Standfirst

	if contentType != BlogType && enrichedContent.Content.MainImage != "" {
		model.ThumbnailURL = new(string)

		var imageID *uuidutils.UUID

		//Generate the actual image UUID from the received image set UUID
		imageSetUUID, err := uuidutils.NewUUIDFromString(enrichedContent.Content.MainImage)
		if err == nil {
			imageID, err = uuidutils.NewUUIDDeriverWith(uuidutils.IMAGE_SET).From(imageSetUUID)
		}

		if err != nil {
			logger.WithError(err).Warnf("Couldn't generate image uuid for the image set with uuid %s: image field won't be populated.", enrichedContent.Content.MainImage)
		} else {
			*model.ThumbnailURL = strings.Replace(imageServiceURL, imagePlaceholder, imageID.String(), -1)
		}

	}

	if contentType == VideoType && len(enrichedContent.Content.DataSources) > 0 {
		for _, ds := range enrichedContent.Content.DataSources {
			if strings.HasPrefix(ds.MediaType, video_prefix) {
				model.LengthMillis = ds.Duration
				break
			}
		}
	}

	if contentType == AudioType && len(enrichedContent.Content.DataSources) > 0 {
		for _, ds := range enrichedContent.Content.DataSources {
			model.LengthMillis = ds.Duration
			break
		}
	}

	model.URL = new(string)
	*model.URL = webURLPrefix + enrichedContent.Content.UUID
	model.ModelAPIURL = new(string)
	*model.ModelAPIURL = fmt.Sprintf("%v%v%v", baseApiUrl, apiURLPrefix, enrichedContent.Content.UUID)
	model.PublishReference = tid
}

func prepareElasticField(elasticField []string, annIDs []string) []string {
	for _, id := range annIDs {
		elasticField = appendIfNotExists(elasticField, id)
	}
	return elasticField
}

func handleSectionMapping(annotation content.Thing, model *content.IndexModel, annIDs []string) {
	// handle sections
	switch annotation.Predicate {
	case about:
		fallthrough
	case majorMentions:
		fallthrough
	case implicitlyAbout:
		fallthrough
	case isClassifiedBy:
		fallthrough
	case implicitlyClassifiedBy:
		model.CmrSections = appendIfNotExists(model.CmrSections, annotation.PrefLabel)
		model.CmrSectionsIds = prepareElasticField(model.CmrSectionsIds, annIDs)
	case isPrimaryClassifiedBy:
		model.CmrSections = appendIfNotExists(model.CmrSections, annotation.PrefLabel)
		model.CmrSectionsIds = prepareElasticField(model.CmrSectionsIds, annIDs)
		model.CmrPrimarysection = new(string)
		*model.CmrPrimarysection = annotation.PrefLabel
		model.CmrPrimarysectionID = new(string)
		*model.CmrPrimarysectionID = getCmrIDWithFallback("Sections", annIDs)
	}
}

func setPrimaryTheme(model *content.IndexModel, name string, id string) {
	if model.CmrPrimarytheme != nil {
		return
	}
	model.CmrPrimarytheme = new(string)
	*model.CmrPrimarytheme = name
	model.CmrPrimarythemeID = new(string)
	*model.CmrPrimarythemeID = id

}

func getCmrID(taxonomy string, annotationIDs []string) (string, bool) {
	encodedTaxonomy := base64.StdEncoding.EncodeToString([]byte(taxonomy))
	for _, annID := range annotationIDs {
		if strings.HasSuffix(annID, encodedTaxonomy) {
			return annID, true
		}
	}
	return "", false
}

func getCmrIDWithFallback(taxonomy string, annotationIDs []string) string {
	encodedTaxonomy := base64.StdEncoding.EncodeToString([]byte(taxonomy))
	for _, annID := range annotationIDs {
		if strings.HasSuffix(annID, encodedTaxonomy) {
			return annID
		}
	}
	if len(annotationIDs) > 1 {
		return annotationIDs[1]
	} else {
		return annotationIDs[0]
	}
}

func appendIfNotExists(s []string, e string) []string {
	for _, a := range s {
		if a == e {
			return s
		}
	}
	return append(s, e)
}
