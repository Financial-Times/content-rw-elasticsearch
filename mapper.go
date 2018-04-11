package main

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/uuid-utils-go"
	"github.com/Financial-Times/content-rw-elasticsearch/mapper/utils"
	"github.com/Financial-Times/content-rw-elasticsearch/content"
)

const (
	isPrimaryClassifiedBy  = "http://www.ft.com/ontology/classification/isPrimarilyClassifiedBy"
	isClassifiedBy         = "http://www.ft.com/ontology/classification/isClassifiedBy"
	implicitlyClassifiedBy = "http://www.ft.com/ontology/implicitlyClassifiedBy"
	about                  = "http://www.ft.com/ontology/annotation/about"
	implicitlyAbout        = "http://www.ft.com/ontology/implicitlyAbout"
	mentions               = "http://www.ft.com/ontology/annotation/mentions"
	hasDisplayTag          = "http://www.ft.com/ontology/hasDisplayTag"
	hasAuthor              = "http://www.ft.com/ontology/annotation/hasAuthor"
	hasContributor         = "http://www.ft.com/ontology/hasContributor"
	webURLPrefix           = "https://www.ft.com/content/"
	apiURLPrefix           = "http://api.ft.com/content/"
	imageServiceURL        = "https://www.ft.com/__origami/service/image/v2/images/raw/http%3A%2F%2Fprod-upp-image-read.ft.com%2F[image_uuid]?source=search&fit=scale-down&width=167"
	imagePlaceholder       = "[image_uuid]"

	tmeOrganisations  = "ON"
	tmePeople         = "PN"
	tmeAuthors        = "Authors"
	tmeBrands         = "Brands"
	tmeSubjects       = "Subjects"
	tmeSections       = "Sections"
	tmeTopics         = "Topics"
	tmeRegions        = "GL"
	tmeGenres         = "Genres"

	ArticleType = "article"
	VideoType   = "video"
	BlogType    = "blog"
)

var ContentTypeMap = map[string]content.ContentType{
	"article": {
		Collection: "FTCom",
		Format:     "Articles",
		Category:   "article",
	},
	"blog": {
		Collection: "FTBlogs",
		Format:     "Blogs",
		Category:   "blogPost",
	},
	"video": {
		Collection: "FTVideos",
		Format:     "Videos",
		Category:   "video",
	},
}

func (indexer *Indexer) ToIndexModel(enrichedContent content.EnrichedContent, contentType string, tid string) content.IndexModel {
	model := content.IndexModel{}

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
		}

		*model.ThumbnailURL = strings.Replace(imageServiceURL, imagePlaceholder, imageID.String(), -1)
	}

	model.URL = new(string)
	*model.URL = webURLPrefix + enrichedContent.Content.UUID
	model.ModelAPIURL = new(string)
	*model.ModelAPIURL = apiURLPrefix + enrichedContent.Content.UUID
	model.PublishReference = tid

	primaryThemeCount := 0
	var ids []string
	var anns []content.Thing
	for _, a := range enrichedContent.Metadata {
		switch a.Thing.Predicate {
		case mentions:
			fallthrough
		case hasDisplayTag:
			//ignore these annotations
			continue
		}
		ids = append(ids, a.Thing.ID)
		anns = append(anns, a.Thing)
	}

	if len(ids) == 0 {
		//TODO confirm
		return model
	}

	concepts, err := indexer.ConceptGetter.GetConcepts(tid, ids)
	if err != nil {
		logger.WithError(err).WithTransactionID(tid).Error(err)
		//TODO confirm
		return model
	}

	for _, annotation := range anns {
		//TODO confirm strip
		fallbackID := strings.TrimPrefix(annotation.ID, thingURIPrefix)
		concordedModel, found := concepts[annotation.ID]
		if !found {
			//TODO is this possible?
			continue
		}
		tmeIDs := []string{fallbackID}
		if concordedModel.TmeIDs != nil {
			tmeIDs = append(tmeIDs, concordedModel.TmeIDs...)
		} else {
			logger.Warnf("Indexing content with uuid %s - TME id missing for concept with id %s, using thing id instead", enrichedContent.Content.UUID, fallbackID)
		}

		handleSectionMapping(annotation, &model, tmeIDs)

		for _, taxonomy := range annotation.Types {
			switch taxonomy {
			case "http://www.ft.com/ontology/organisation/Organisation":
				model.CmrOrgnames = appendIfNotExists(model.CmrOrgnames, annotation.PrefLabel)
				model.CmrOrgnamesIds = appendIfNotExists(model.CmrOrgnamesIds, getCmrID(tmeOrganisations, tmeIDs))
				if annotation.Predicate == about {
					setPrimaryTheme(&model, &primaryThemeCount, annotation.PrefLabel, getCmrID(tmeOrganisations, tmeIDs))
				}
			case "http://www.ft.com/ontology/person/Person":
				cmrID := getCmrID(tmePeople, tmeIDs)
				authorCmrID := getCmrID(tmeAuthors, tmeIDs)
				// if it's only author, skip adding to people
				if cmrID != fallbackID || authorCmrID == fallbackID {
					model.CmrPeople = appendIfNotExists(model.CmrPeople, annotation.PrefLabel)
					model.CmrPeopleIds = appendIfNotExists(model.CmrPeopleIds, cmrID)
				}
				if annotation.Predicate == hasAuthor || annotation.Predicate == hasContributor {
					if authorCmrID != fallbackID {
						model.CmrAuthors = appendIfNotExists(model.CmrAuthors, annotation.PrefLabel)
						model.CmrAuthorsIds = appendIfNotExists(model.CmrAuthorsIds, authorCmrID)
					}
				}
				if annotation.Predicate == about {
					setPrimaryTheme(&model, &primaryThemeCount, annotation.PrefLabel, getCmrID(tmePeople, tmeIDs))
				}
			case "http://www.ft.com/ontology/company/Company":
				model.CmrCompanynames = appendIfNotExists(model.CmrCompanynames, annotation.PrefLabel)
				model.CmrCompanynamesIds = appendIfNotExists(model.CmrCompanynamesIds, getCmrID(tmeOrganisations, tmeIDs))
			case "http://www.ft.com/ontology/product/Brand":
				model.CmrBrands = appendIfNotExists(model.CmrBrands, annotation.PrefLabel)
				model.CmrBrandsIds = appendIfNotExists(model.CmrBrandsIds, getCmrID(tmeBrands, tmeIDs))
			case "http://www.ft.com/ontology/Subject":
				model.CmrSubjects = appendIfNotExists(model.CmrSubjects, annotation.PrefLabel)
				model.CmrSubjectsIds = appendIfNotExists(model.CmrSubjectsIds, getCmrID(tmeSubjects, tmeIDs))
			case "http://www.ft.com/ontology/Topic":
				model.CmrTopics = appendIfNotExists(model.CmrTopics, annotation.PrefLabel)
				model.CmrTopicsIds = appendIfNotExists(model.CmrTopicsIds, getCmrID(tmeTopics, tmeIDs))
				if annotation.Predicate == about {
					setPrimaryTheme(&model, &primaryThemeCount, annotation.PrefLabel, getCmrID(tmeTopics, tmeIDs))
				}
			case "http://www.ft.com/ontology/Location":
				model.CmrRegions = appendIfNotExists(model.CmrRegions, annotation.PrefLabel)
				model.CmrRegionsIds = appendIfNotExists(model.CmrRegionsIds, getCmrID(tmeRegions, tmeIDs))
				if annotation.Predicate == about {
					setPrimaryTheme(&model, &primaryThemeCount, annotation.PrefLabel, getCmrID(tmeRegions, tmeIDs))
				}
			case "http://www.ft.com/ontology/Genre":
				model.CmrGenres = appendIfNotExists(model.CmrGenres, annotation.PrefLabel)
				model.CmrGenreIds = appendIfNotExists(model.CmrGenreIds, getCmrID(tmeGenres, tmeIDs))
			}
		}
	}
	return model
}

func handleSectionMapping(annotation content.Thing, model *content.IndexModel, tmeIDs []string) {
	// handle sections
	switch annotation.Predicate {
	case about:
		fallthrough
	case implicitlyAbout:
		fallthrough
	case isClassifiedBy:
		fallthrough
	case implicitlyClassifiedBy:
		model.CmrSections = appendIfNotExists(model.CmrSections, annotation.PrefLabel)
		model.CmrSectionsIds = appendIfNotExists(model.CmrSectionsIds, getCmrID(tmeSections, tmeIDs))
	case isPrimaryClassifiedBy:
		model.CmrPrimarysection = new(string)
		*model.CmrPrimarysection = annotation.PrefLabel
		model.CmrPrimarysectionID = new(string)
		*model.CmrPrimarysectionID = getCmrID(tmeSections, tmeIDs)
	}
}
func setPrimaryTheme(model *content.IndexModel, pTCount *int, name string, id string) {
	if *pTCount == 0 {
		model.CmrPrimarytheme = new(string)
		*model.CmrPrimarytheme = name
		model.CmrPrimarythemeID = new(string)
		*model.CmrPrimarythemeID = id
	} else {
		model.CmrPrimarytheme = nil
		model.CmrPrimarythemeID = nil
	}
	*pTCount++
}

func getCmrID(taxonomy string, tmeIDs []string) string {
	encodedTaxonomy := base64.StdEncoding.EncodeToString([]byte(taxonomy))
	for _, tmeID := range tmeIDs {
		if strings.HasSuffix(tmeID, encodedTaxonomy) {
			return tmeID
		}
	}
	return tmeIDs[0]
}

func appendIfNotExists(s []string, e string) []string {
	for _, a := range s {
		if a == e {
			return s
		}
	}
	return append(s, e)
}
