package sarin

import (
	"bytes"
	"encoding/base64"
	"math/rand/v2"
	"mime/multipart"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"go.aykhans.me/sarin/internal/types"
)

func NewDefaultTemplateFuncMap(randSource rand.Source, fileCache *FileCache) template.FuncMap {
	fakeit := gofakeit.NewFaker(randSource, false)

	return template.FuncMap{
		// Strings
		"strings_ToUpper":      strings.ToUpper,
		"strings_ToLower":      strings.ToLower,
		"strings_RemoveSpaces": func(s string) string { return strings.ReplaceAll(s, " ", "") },
		"strings_Replace":      strings.Replace,
		"strings_ToDate": func(dateString string) time.Time {
			date, err := time.Parse("2006-01-02", dateString)
			if err != nil {
				return time.Now()
			}
			return date
		},
		"strings_First": func(s string, n int) string {
			runes := []rune(s)
			if n <= 0 {
				return ""
			}
			if n >= len(runes) {
				return s
			}
			return string(runes[:n])
		},
		"strings_Last": func(s string, n int) string {
			runes := []rune(s)
			if n <= 0 {
				return ""
			}
			if n >= len(runes) {
				return s
			}
			return string(runes[len(runes)-n:])
		},
		"strings_Truncate": func(s string, n int) string {
			runes := []rune(s)
			if n <= 0 {
				return "..."
			}
			if n >= len(runes) {
				return s
			}
			return string(runes[:n]) + "..."
		},
		"strings_TrimPrefix": strings.TrimPrefix,
		"strings_TrimSuffix": strings.TrimSuffix,
		// Dict
		"dict_Str": func(values ...string) map[string]string {
			dict := make(map[string]string)
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					key := values[i]
					value := values[i+1]
					dict[key] = value
				}
			}
			return dict
		},

		// Slice
		"slice_Str":  func(values ...string) []string { return values },
		"slice_Int":  func(values ...int) []int { return values },
		"slice_Uint": func(values ...uint) []uint { return values },
		"slice_Join": strings.Join,

		// File
		// file_Base64 reads a file (local or remote URL) and returns its Base64 encoded content.
		// Usage: {{ file_Base64 "/path/to/file.pdf" }}
		//        {{ file_Base64 "https://example.com/image.png" }}
		"file_Base64": func(source string) (string, error) {
			if fileCache == nil {
				return "", types.ErrFileCacheNotInitialized
			}
			cached, err := fileCache.GetOrLoad(source)
			if err != nil {
				return "", err
			}
			return base64.StdEncoding.EncodeToString(cached.Content), nil
		},

		// Fakeit / File
		// "fakeit_CSV": fakeit.CSV(nil),
		// "fakeit_JSON": fakeit.JSON(nil),
		// "fakeit_XML": fakeit.XML(nil),
		"fakeit_FileExtension": fakeit.FileExtension,
		"fakeit_FileMimeType":  fakeit.FileMimeType,

		// Fakeit / ID
		"fakeit_ID":   fakeit.ID,
		"fakeit_UUID": fakeit.UUID,

		// Fakeit / Template
		// "fakeit_Template": fakeit.Template(nil) (string, error),
		// "fakeit_Markdown": fakeit.Markdown(nil) (string, error),
		// "fakeit_EmailText": fakeit.EmailText(nil) (string, error),
		// "fakeit_FixedWidth": fakeit.FixedWidth(nil) (string, error),

		// Fakeit / Product
		// "fakeit_Product": fakeit.Product() *ProductInfo,
		"fakeit_ProductName":        fakeit.ProductName,
		"fakeit_ProductDescription": fakeit.ProductDescription,
		"fakeit_ProductCategory":    fakeit.ProductCategory,
		"fakeit_ProductFeature":     fakeit.ProductFeature,
		"fakeit_ProductMaterial":    fakeit.ProductMaterial,
		"fakeit_ProductUPC":         fakeit.ProductUPC,
		"fakeit_ProductAudience":    fakeit.ProductAudience,
		"fakeit_ProductDimension":   fakeit.ProductDimension,
		"fakeit_ProductUseCase":     fakeit.ProductUseCase,
		"fakeit_ProductBenefit":     fakeit.ProductBenefit,
		"fakeit_ProductSuffix":      fakeit.ProductSuffix,
		"fakeit_ProductISBN":        func() string { return fakeit.ProductISBN(nil) },

		// Fakeit / Person
		// "fakeit_Person": fakeit.Person() *PersonInfo,
		"fakeit_Name":       fakeit.Name,
		"fakeit_NamePrefix": fakeit.NamePrefix,
		"fakeit_NameSuffix": fakeit.NameSuffix,
		"fakeit_FirstName":  fakeit.FirstName,
		"fakeit_MiddleName": fakeit.MiddleName,
		"fakeit_LastName":   fakeit.LastName,
		"fakeit_Gender":     fakeit.Gender,
		"fakeit_Age":        fakeit.Age,
		"fakeit_Ethnicity":  fakeit.Ethnicity,
		"fakeit_SSN":        fakeit.SSN,
		"fakeit_EIN":        fakeit.EIN,
		"fakeit_Hobby":      fakeit.Hobby,
		// "fakeit_Contact": fakeit.Contact() *ContactInfo,
		"fakeit_Email":          fakeit.Email,
		"fakeit_Phone":          fakeit.Phone,
		"fakeit_PhoneFormatted": fakeit.PhoneFormatted,
		// "fakeit_Teams": fakeit.Teams(peopleArray []string, teamsArray []string) map[string][]string,

		// Fakeit / Generate
		// "fakeit_Struct": fakeit.Struct(v any),
		// "fakeit_Slice": fakeit.Slice(v any),
		// "fakeit_Map": fakeit.Map() map[string]any,
		// "fakeit_Generate": fakeit.Generate(value string) string,
		"fakeit_Regex": fakeit.Regex,

		// Fakeit / Auth
		"fakeit_Username": fakeit.Username,
		"fakeit_Password": fakeit.Password,

		// Fakeit / Address
		// "fakeit_Address": fakeit.Address() *AddressInfo,
		"fakeit_City":         fakeit.City,
		"fakeit_Country":      fakeit.Country,
		"fakeit_CountryAbr":   fakeit.CountryAbr,
		"fakeit_State":        fakeit.State,
		"fakeit_StateAbr":     fakeit.StateAbr,
		"fakeit_Street":       fakeit.Street,
		"fakeit_StreetName":   fakeit.StreetName,
		"fakeit_StreetNumber": fakeit.StreetNumber,
		"fakeit_StreetPrefix": fakeit.StreetPrefix,
		"fakeit_StreetSuffix": fakeit.StreetSuffix,
		"fakeit_Unit":         fakeit.Unit,
		"fakeit_Zip":          fakeit.Zip,
		"fakeit_Latitude":     fakeit.Latitude,
		"fakeit_LatitudeInRange": func(minLatitude, maxLatitude float64) float64 {
			value, err := fakeit.LatitudeInRange(minLatitude, maxLatitude)
			if err != nil {
				var zero float64
				return zero
			}
			return value
		},
		"fakeit_Longitude": fakeit.Longitude,
		"fakeit_LongitudeInRange": func(minLongitude, maxLongitude float64) float64 {
			value, err := fakeit.LongitudeInRange(minLongitude, maxLongitude)
			if err != nil {
				var zero float64
				return zero
			}
			return value
		},

		// Fakeit / Game
		"fakeit_Gamertag": fakeit.Gamertag,
		// "fakeit_Dice": fakeit.Dice(numDice uint, sides []uint) []uint,

		// Fakeit / Beer
		"fakeit_BeerAlcohol": fakeit.BeerAlcohol,
		"fakeit_BeerBlg":     fakeit.BeerBlg,
		"fakeit_BeerHop":     fakeit.BeerHop,
		"fakeit_BeerIbu":     fakeit.BeerIbu,
		"fakeit_BeerMalt":    fakeit.BeerMalt,
		"fakeit_BeerName":    fakeit.BeerName,
		"fakeit_BeerStyle":   fakeit.BeerStyle,
		"fakeit_BeerYeast":   fakeit.BeerYeast,

		// Fakeit / Car
		// "fakeit_Car": fakeit.Car() *CarInfo,
		"fakeit_CarMaker":            fakeit.CarMaker,
		"fakeit_CarModel":            fakeit.CarModel,
		"fakeit_CarType":             fakeit.CarType,
		"fakeit_CarFuelType":         fakeit.CarFuelType,
		"fakeit_CarTransmissionType": fakeit.CarTransmissionType,

		// Fakeit / Words
		// Nouns
		"fakeit_Noun":                 fakeit.Noun,
		"fakeit_NounCommon":           fakeit.NounCommon,
		"fakeit_NounConcrete":         fakeit.NounConcrete,
		"fakeit_NounAbstract":         fakeit.NounAbstract,
		"fakeit_NounCollectivePeople": fakeit.NounCollectivePeople,
		"fakeit_NounCollectiveAnimal": fakeit.NounCollectiveAnimal,
		"fakeit_NounCollectiveThing":  fakeit.NounCollectiveThing,
		"fakeit_NounCountable":        fakeit.NounCountable,
		"fakeit_NounUncountable":      fakeit.NounUncountable,

		// Verbs
		"fakeit_Verb":        fakeit.Verb,
		"fakeit_VerbAction":  fakeit.VerbAction,
		"fakeit_VerbLinking": fakeit.VerbLinking,
		"fakeit_VerbHelping": fakeit.VerbHelping,

		// Adverbs
		"fakeit_Adverb":                    fakeit.Adverb,
		"fakeit_AdverbManner":              fakeit.AdverbManner,
		"fakeit_AdverbDegree":              fakeit.AdverbDegree,
		"fakeit_AdverbPlace":               fakeit.AdverbPlace,
		"fakeit_AdverbTimeDefinite":        fakeit.AdverbTimeDefinite,
		"fakeit_AdverbTimeIndefinite":      fakeit.AdverbTimeIndefinite,
		"fakeit_AdverbFrequencyDefinite":   fakeit.AdverbFrequencyDefinite,
		"fakeit_AdverbFrequencyIndefinite": fakeit.AdverbFrequencyIndefinite,

		// Propositions
		"fakeit_Preposition":         fakeit.Preposition,
		"fakeit_PrepositionSimple":   fakeit.PrepositionSimple,
		"fakeit_PrepositionDouble":   fakeit.PrepositionDouble,
		"fakeit_PrepositionCompound": fakeit.PrepositionCompound,

		// Adjectives
		"fakeit_Adjective":              fakeit.Adjective,
		"fakeit_AdjectiveDescriptive":   fakeit.AdjectiveDescriptive,
		"fakeit_AdjectiveQuantitative":  fakeit.AdjectiveQuantitative,
		"fakeit_AdjectiveProper":        fakeit.AdjectiveProper,
		"fakeit_AdjectiveDemonstrative": fakeit.AdjectiveDemonstrative,
		"fakeit_AdjectivePossessive":    fakeit.AdjectivePossessive,
		"fakeit_AdjectiveInterrogative": fakeit.AdjectiveInterrogative,
		"fakeit_AdjectiveIndefinite":    fakeit.AdjectiveIndefinite,

		// Pronouns
		"fakeit_Pronoun":              fakeit.Pronoun,
		"fakeit_PronounPersonal":      fakeit.PronounPersonal,
		"fakeit_PronounObject":        fakeit.PronounObject,
		"fakeit_PronounPossessive":    fakeit.PronounPossessive,
		"fakeit_PronounReflective":    fakeit.PronounReflective,
		"fakeit_PronounDemonstrative": fakeit.PronounDemonstrative,
		"fakeit_PronounInterrogative": fakeit.PronounInterrogative,
		"fakeit_PronounRelative":      fakeit.PronounRelative,

		// Connectives
		"fakeit_Connective":            fakeit.Connective,
		"fakeit_ConnectiveTime":        fakeit.ConnectiveTime,
		"fakeit_ConnectiveComparative": fakeit.ConnectiveComparative,
		"fakeit_ConnectiveComplaint":   fakeit.ConnectiveComplaint,
		"fakeit_ConnectiveListing":     fakeit.ConnectiveListing,
		"fakeit_ConnectiveCasual":      fakeit.ConnectiveCasual,
		"fakeit_ConnectiveExamplify":   fakeit.ConnectiveExamplify,

		// Words
		"fakeit_Word": fakeit.Word,

		// Text
		"fakeit_Sentence":            fakeit.Sentence,
		"fakeit_Paragraph":           fakeit.Paragraph,
		"fakeit_LoremIpsumWord":      fakeit.LoremIpsumWord,
		"fakeit_LoremIpsumSentence":  fakeit.LoremIpsumSentence,
		"fakeit_LoremIpsumParagraph": fakeit.LoremIpsumParagraph,
		"fakeit_Question":            fakeit.Question,
		"fakeit_Quote":               fakeit.Quote,
		"fakeit_Phrase":              fakeit.Phrase,

		// Fakeit / Foods
		"fakeit_Fruit":     fakeit.Fruit,
		"fakeit_Vegetable": fakeit.Vegetable,
		"fakeit_Breakfast": fakeit.Breakfast,
		"fakeit_Lunch":     fakeit.Lunch,
		"fakeit_Dinner":    fakeit.Dinner,
		"fakeit_Snack":     fakeit.Snack,
		"fakeit_Dessert":   fakeit.Dessert,

		// Fakeit / Misc
		"fakeit_Bool": fakeit.Bool,
		// "fakeit_Weighted": fakeit.Weighted(options []any, weights []float32) (any, error),
		"fakeit_FlipACoin": fakeit.FlipACoin,
		// "fakeit_RandomMapKey": fakeit.RandomMapKey(mapI any) any,
		// "fakeit_ShuffleAnySlice": fakeit.ShuffleAnySlice(v any),

		// Fakeit / Colors
		"fakeit_Color":      fakeit.Color,
		"fakeit_HexColor":   fakeit.HexColor,
		"fakeit_RGBColor":   fakeit.RGBColor,
		"fakeit_SafeColor":  fakeit.SafeColor,
		"fakeit_NiceColors": fakeit.NiceColors,

		// Fakeit / Images
		// "fakeit_Image": fakeit.Image(width int, height int) *img.RGBA,
		"fakeit_ImageJpeg": fakeit.ImageJpeg,
		"fakeit_ImagePng":  fakeit.ImagePng,

		// Fakeit / Internet
		"fakeit_URL":                  fakeit.URL,
		"fakeit_UrlSlug":              fakeit.UrlSlug,
		"fakeit_DomainName":           fakeit.DomainName,
		"fakeit_DomainSuffix":         fakeit.DomainSuffix,
		"fakeit_IPv4Address":          fakeit.IPv4Address,
		"fakeit_IPv6Address":          fakeit.IPv6Address,
		"fakeit_MacAddress":           fakeit.MacAddress,
		"fakeit_HTTPStatusCode":       fakeit.HTTPStatusCode,
		"fakeit_HTTPStatusCodeSimple": fakeit.HTTPStatusCodeSimple,
		"fakeit_LogLevel":             fakeit.LogLevel,
		"fakeit_HTTPMethod":           fakeit.HTTPMethod,
		"fakeit_HTTPVersion":          fakeit.HTTPVersion,
		"fakeit_UserAgent":            fakeit.UserAgent,
		"fakeit_ChromeUserAgent":      fakeit.ChromeUserAgent,
		"fakeit_FirefoxUserAgent":     fakeit.FirefoxUserAgent,
		"fakeit_OperaUserAgent":       fakeit.OperaUserAgent,
		"fakeit_SafariUserAgent":      fakeit.SafariUserAgent,
		"fakeit_APIUserAgent":         fakeit.APIUserAgent,

		// Fakeit / HTML
		"fakeit_InputName": fakeit.InputName,
		"fakeit_Svg":       func() string { return fakeit.Svg(nil) },

		// Fakeit / Date/Time
		"fakeit_Date":           fakeit.Date,
		"fakeit_PastDate":       fakeit.PastDate,
		"fakeit_FutureDate":     fakeit.FutureDate,
		"fakeit_DateRange":      fakeit.DateRange,
		"fakeit_NanoSecond":     fakeit.NanoSecond,
		"fakeit_Second":         fakeit.Second,
		"fakeit_Minute":         fakeit.Minute,
		"fakeit_Hour":           fakeit.Hour,
		"fakeit_Month":          fakeit.Month,
		"fakeit_MonthString":    fakeit.MonthString,
		"fakeit_Day":            fakeit.Day,
		"fakeit_WeekDay":        fakeit.WeekDay,
		"fakeit_Year":           fakeit.Year,
		"fakeit_TimeZone":       fakeit.TimeZone,
		"fakeit_TimeZoneAbv":    fakeit.TimeZoneAbv,
		"fakeit_TimeZoneFull":   fakeit.TimeZoneFull,
		"fakeit_TimeZoneOffset": fakeit.TimeZoneOffset,
		"fakeit_TimeZoneRegion": fakeit.TimeZoneRegion,

		// Fakeit / Payment
		"fakeit_Price": fakeit.Price,
		// "fakeit_CreditCard": fakeit.CreditCard() *CreditCardInfo,
		"fakeit_CreditCardCvv": fakeit.CreditCardCvv,
		"fakeit_CreditCardExp": fakeit.CreditCardExp,
		"fakeit_CreditCardNumber": func(gaps bool) string {
			return fakeit.CreditCardNumber(&gofakeit.CreditCardOptions{Gaps: gaps})
		},
		"fakeit_CreditCardType": fakeit.CreditCardType,
		// "fakeit_Currency": fakeit.Currency() *CurrencyInfo,
		"fakeit_CurrencyLong":      fakeit.CurrencyLong,
		"fakeit_CurrencyShort":     fakeit.CurrencyShort,
		"fakeit_AchRouting":        fakeit.AchRouting,
		"fakeit_AchAccount":        fakeit.AchAccount,
		"fakeit_BitcoinAddress":    fakeit.BitcoinAddress,
		"fakeit_BitcoinPrivateKey": fakeit.BitcoinPrivateKey,
		"fakeit_BankName":          fakeit.BankName,
		"fakeit_BankType":          fakeit.BankType,

		// Fakeit / Finance
		"fakeit_Cusip": fakeit.Cusip,
		"fakeit_Isin":  fakeit.Isin,

		// Fakeit / Company
		"fakeit_BS":            fakeit.BS,
		"fakeit_Blurb":         fakeit.Blurb,
		"fakeit_BuzzWord":      fakeit.BuzzWord,
		"fakeit_Company":       fakeit.Company,
		"fakeit_CompanySuffix": fakeit.CompanySuffix,
		// "fakeit_Job": fakeit.Job() *JobInfo,
		"fakeit_JobDescriptor": fakeit.JobDescriptor,
		"fakeit_JobLevel":      fakeit.JobLevel,
		"fakeit_JobTitle":      fakeit.JobTitle,
		"fakeit_Slogan":        fakeit.Slogan,

		// Fakeit / Hacker
		"fakeit_HackerAbbreviation": fakeit.HackerAbbreviation,
		"fakeit_HackerAdjective":    fakeit.HackerAdjective,
		"fakeit_HackeringVerb":      fakeit.HackeringVerb,
		"fakeit_HackerNoun":         fakeit.HackerNoun,
		"fakeit_HackerPhrase":       fakeit.HackerPhrase,
		"fakeit_HackerVerb":         fakeit.HackerVerb,

		// Fakeit / Hipster
		"fakeit_HipsterWord":      fakeit.HipsterWord,
		"fakeit_HipsterSentence":  fakeit.HipsterSentence,
		"fakeit_HipsterParagraph": fakeit.HipsterParagraph,

		// Fakeit / App
		"fakeit_AppName":    fakeit.AppName,
		"fakeit_AppVersion": fakeit.AppVersion,
		"fakeit_AppAuthor":  fakeit.AppAuthor,

		// Fakeit / Animal
		"fakeit_PetName":    fakeit.PetName,
		"fakeit_Animal":     fakeit.Animal,
		"fakeit_AnimalType": fakeit.AnimalType,
		"fakeit_FarmAnimal": fakeit.FarmAnimal,
		"fakeit_Cat":        fakeit.Cat,
		"fakeit_Dog":        fakeit.Dog,
		"fakeit_Bird":       fakeit.Bird,

		// Fakeit / Emoji
		"fakeit_Emoji":            fakeit.Emoji,
		"fakeit_EmojiCategory":    fakeit.EmojiCategory,
		"fakeit_EmojiAlias":       fakeit.EmojiAlias,
		"fakeit_EmojiTag":         fakeit.EmojiTag,
		"fakeit_EmojiFlag":        fakeit.EmojiFlag,
		"fakeit_EmojiAnimal":      fakeit.EmojiAnimal,
		"fakeit_EmojiFood":        fakeit.EmojiFood,
		"fakeit_EmojiPlant":       fakeit.EmojiPlant,
		"fakeit_EmojiMusic":       fakeit.EmojiMusic,
		"fakeit_EmojiVehicle":     fakeit.EmojiVehicle,
		"fakeit_EmojiSport":       fakeit.EmojiSport,
		"fakeit_EmojiFace":        fakeit.EmojiFace,
		"fakeit_EmojiHand":        fakeit.EmojiHand,
		"fakeit_EmojiClothing":    fakeit.EmojiClothing,
		"fakeit_EmojiLandmark":    fakeit.EmojiLandmark,
		"fakeit_EmojiElectronics": fakeit.EmojiElectronics,
		"fakeit_EmojiGame":        fakeit.EmojiGame,
		"fakeit_EmojiTools":       fakeit.EmojiTools,
		"fakeit_EmojiWeather":     fakeit.EmojiWeather,
		"fakeit_EmojiJob":         fakeit.EmojiJob,
		"fakeit_EmojiPerson":      fakeit.EmojiPerson,
		"fakeit_EmojiGesture":     fakeit.EmojiGesture,
		"fakeit_EmojiCostume":     fakeit.EmojiCostume,
		"fakeit_EmojiSentence":    fakeit.EmojiSentence,

		// Fakeit / Language
		"fakeit_Language":             fakeit.Language,
		"fakeit_LanguageAbbreviation": fakeit.LanguageAbbreviation,
		"fakeit_ProgrammingLanguage":  fakeit.ProgrammingLanguage,

		// Fakeit / Number
		"fakeit_Number":       fakeit.Number,
		"fakeit_Int":          fakeit.Int,
		"fakeit_IntN":         fakeit.IntN,
		"fakeit_Int8":         fakeit.Int8,
		"fakeit_Int16":        fakeit.Int16,
		"fakeit_Int32":        fakeit.Int32,
		"fakeit_Int64":        fakeit.Int64,
		"fakeit_Uint":         fakeit.Uint,
		"fakeit_UintN":        fakeit.UintN,
		"fakeit_Uint8":        fakeit.Uint8,
		"fakeit_Uint16":       fakeit.Uint16,
		"fakeit_Uint32":       fakeit.Uint32,
		"fakeit_Uint64":       fakeit.Uint64,
		"fakeit_Float32":      fakeit.Float32,
		"fakeit_Float32Range": fakeit.Float32Range,
		"fakeit_Float64":      fakeit.Float64,
		"fakeit_Float64Range": fakeit.Float64Range,
		// "fakeit_ShuffleInts":  fakeit.ShuffleInts,
		"fakeit_RandomInt": fakeit.RandomInt,
		"fakeit_HexUint":   fakeit.HexUint,

		// Fakeit / String
		"fakeit_Digit":    fakeit.Digit,
		"fakeit_DigitN":   fakeit.DigitN,
		"fakeit_Letter":   fakeit.Letter,
		"fakeit_LetterN":  fakeit.LetterN,
		"fakeit_Lexify":   fakeit.Lexify,
		"fakeit_Numerify": fakeit.Numerify,
		// "fakeit_ShuffleStrings": fakeit.ShuffleStrings,
		"fakeit_RandomString": fakeit.RandomString,

		// Fakeit / Celebrity
		"fakeit_CelebrityActor":    fakeit.CelebrityActor,
		"fakeit_CelebrityBusiness": fakeit.CelebrityBusiness,
		"fakeit_CelebritySport":    fakeit.CelebritySport,

		// Fakeit / Minecraft
		"fakeit_MinecraftOre":             fakeit.MinecraftOre,
		"fakeit_MinecraftWood":            fakeit.MinecraftWood,
		"fakeit_MinecraftArmorTier":       fakeit.MinecraftArmorTier,
		"fakeit_MinecraftArmorPart":       fakeit.MinecraftArmorPart,
		"fakeit_MinecraftWeapon":          fakeit.MinecraftWeapon,
		"fakeit_MinecraftTool":            fakeit.MinecraftTool,
		"fakeit_MinecraftDye":             fakeit.MinecraftDye,
		"fakeit_MinecraftFood":            fakeit.MinecraftFood,
		"fakeit_MinecraftAnimal":          fakeit.MinecraftAnimal,
		"fakeit_MinecraftVillagerJob":     fakeit.MinecraftVillagerJob,
		"fakeit_MinecraftVillagerStation": fakeit.MinecraftVillagerStation,
		"fakeit_MinecraftVillagerLevel":   fakeit.MinecraftVillagerLevel,
		"fakeit_MinecraftMobPassive":      fakeit.MinecraftMobPassive,
		"fakeit_MinecraftMobNeutral":      fakeit.MinecraftMobNeutral,
		"fakeit_MinecraftMobHostile":      fakeit.MinecraftMobHostile,
		"fakeit_MinecraftMobBoss":         fakeit.MinecraftMobBoss,
		"fakeit_MinecraftBiome":           fakeit.MinecraftBiome,
		"fakeit_MinecraftWeather":         fakeit.MinecraftWeather,

		// Fakeit / Book
		// "fakeit_Book": fakeit.Book() *BookInfo,
		"fakeit_BookTitle":  fakeit.BookTitle,
		"fakeit_BookAuthor": fakeit.BookAuthor,
		"fakeit_BookGenre":  fakeit.BookGenre,

		// Fakeit / Movie
		// "fakeit_Movie": fakeit.Movie() *MovieInfo,
		"fakeit_MovieName":  fakeit.MovieName,
		"fakeit_MovieGenre": fakeit.MovieGenre,

		// Fakeit / Error
		"fakeit_Error":           func() string { return fakeit.Error().Error() },
		"fakeit_ErrorDatabase":   func() string { return fakeit.ErrorDatabase().Error() },
		"fakeit_ErrorGRPC":       func() string { return fakeit.ErrorGRPC().Error() },
		"fakeit_ErrorHTTP":       func() string { return fakeit.ErrorHTTP().Error() },
		"fakeit_ErrorHTTPClient": func() string { return fakeit.ErrorHTTPClient().Error() },
		"fakeit_ErrorHTTPServer": func() string { return fakeit.ErrorHTTPServer().Error() },
		// "fakeit_ErrorInput": func() string { return fakeit.ErrorInput().Error() },
		"fakeit_ErrorRuntime": func() string { return fakeit.ErrorRuntime().Error() },

		// Fakeit / School
		"fakeit_School": fakeit.School,

		// Fakeit / Song
		// "fakeit_Song": fakeit.Song() *SongInfo,
		"fakeit_SongName":   fakeit.SongName,
		"fakeit_SongArtist": fakeit.SongArtist,
		"fakeit_SongGenre":  fakeit.SongGenre,
	}
}

type BodyTemplateFuncMapData struct {
	formDataContenType string
}

func (data BodyTemplateFuncMapData) GetFormDataContenType() string {
	return data.formDataContenType
}

func (data *BodyTemplateFuncMapData) ClearFormDataContenType() {
	data.formDataContenType = ""
}

func NewDefaultBodyTemplateFuncMap(
	randSource rand.Source,
	data *BodyTemplateFuncMapData,
	fileCache *FileCache,
) template.FuncMap {
	funcMap := NewDefaultTemplateFuncMap(randSource, fileCache)

	if data != nil {
		// body_FormData creates a multipart/form-data body from key-value pairs.
		// Usage: {{ body_FormData "field1" "value1" "field2" "value2" ... }}
		//
		// Values starting with "@" are treated as file references:
		//   - "@/path/to/file.txt" - local file
		//   - "@http://example.com/file" - remote file via HTTP
		//   - "@https://example.com/file" - remote file via HTTPS
		//
		// To send a literal string starting with "@", escape it with "@@":
		//   - "@@literal" sends "@literal"
		//
		// Example with mixed text and files:
		//   {{ body_FormData "name" "John" "avatar" "@/path/to/photo.jpg" "doc" "@https://example.com/file.pdf" }}
		funcMap["body_FormData"] = func(pairs ...string) (string, error) {
			if len(pairs)%2 != 0 {
				return "", types.ErrFormDataOddArgs
			}

			var multipartData bytes.Buffer
			writer := multipart.NewWriter(&multipartData)
			data.formDataContenType = writer.FormDataContentType()

			for i := 0; i < len(pairs); i += 2 {
				key := pairs[i]
				val := pairs[i+1]

				switch {
				case strings.HasPrefix(val, "@@"):
					// Escaped @ - send as literal string without first @
					if err := writer.WriteField(key, val[1:]); err != nil {
						return "", err
					}
				case strings.HasPrefix(val, "@"):
					// File (local path or remote URL)
					if fileCache == nil {
						return "", types.ErrFileCacheNotInitialized
					}
					source := val[1:]
					cached, err := fileCache.GetOrLoad(source)
					if err != nil {
						return "", err
					}
					part, err := writer.CreateFormFile(key, cached.Filename)
					if err != nil {
						return "", err
					}
					if _, err := part.Write(cached.Content); err != nil {
						return "", err
					}
				default:
					// Regular text field
					if err := writer.WriteField(key, val); err != nil {
						return "", err
					}
				}
			}

			if err := writer.Close(); err != nil {
				return "", err
			}
			return multipartData.String(), nil
		}
	}

	return funcMap
}

func hasTemplateActions(tmpl *template.Template) bool {
	if tmpl.Tree == nil || tmpl.Root == nil {
		return false
	}

	for _, node := range tmpl.Root.Nodes {
		switch node.Type() {
		case parse.NodeAction, parse.NodeIf, parse.NodeRange,
			parse.NodeWith, parse.NodeTemplate:
			return true
		}
	}
	return false
}
