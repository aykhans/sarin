# Templating

Sarin supports Go templates in URL paths, methods, bodies, headers, params, cookies, and values.

> **Note:** Templating in URL host and scheme is not supported. Only the path portion of the URL can contain templates.

## Table of Contents

- [Using Values](#using-values)
- [General Functions](#general-functions)
    - [String Functions](#string-functions)
    - [Collection Functions](#collection-functions)
    - [Body Functions](#body-functions)
    - [File Functions](#file-functions)
- [Fake Data Functions](#fake-data-functions)
    - [File](#file)
    - [ID](#id)
    - [Product](#product)
    - [Person](#person)
    - [Generate](#generate)
    - [Auth](#auth)
    - [Address](#address)
    - [Game](#game)
    - [Beer](#beer)
    - [Car](#car)
    - [Words](#words)
    - [Text](#text)
    - [Foods](#foods)
    - [Misc](#misc)
    - [Color](#color)
    - [Image](#image)
    - [Internet](#internet)
    - [HTML](#html)
    - [Date/Time](#datetime)
    - [Payment](#payment)
    - [Finance](#finance)
    - [Company](#company)
    - [Hacker](#hacker)
    - [Hipster](#hipster)
    - [App](#app)
    - [Animal](#animal)
    - [Emoji](#emoji)
    - [Language](#language)
    - [Number](#number)
    - [String](#string)
    - [Celebrity](#celebrity)
    - [Minecraft](#minecraft)
    - [Book](#book)
    - [Movie](#movie)
    - [Error](#error)
    - [School](#school)
    - [Song](#song)

## Using Values

Values are generated once per request and can be referenced in multiple fields using `{{ .Values.KEY }}` syntax. This is useful when you need to use the same generated value (e.g., a UUID) in both headers and body within the same request.

**Example:**

```yaml
values: |
    REQUEST_ID={{ fakeit_UUID }}
    USER_ID={{ fakeit_UUID }}

headers:
    X-Request-ID: "{{ .Values.REQUEST_ID }}"
body: |
    {
      "requestId": "{{ .Values.REQUEST_ID }}",
      "userId": "{{ .Values.USER_ID }}"
    }
```

In this example, `REQUEST_ID` is generated once and the same value is used in both the header and body. Each new request generates a new `REQUEST_ID`.

**CLI example:**

```sh
sarin -U http://example.com/users \
  -V "ID={{ fakeit_UUID }}" \
  -H "X-Request-ID: {{ .Values.ID }}" \
  -B '{"id": "{{ .Values.ID }}"}'
```

## General Functions

### String Functions

| Function                                                   | Description                                                         | Example                                                   |
| ---------------------------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------- |
| `strings_ToUpper`                                          | Convert string to uppercase                                         | `{{ strings_ToUpper "hello" }}` → `HELLO`                 |
| `strings_ToLower`                                          | Convert string to lowercase                                         | `{{ strings_ToLower "HELLO" }}` → `hello`                 |
| `strings_RemoveSpaces`                                     | Remove all spaces from string                                       | `{{ strings_RemoveSpaces "hello world" }}` → `helloworld` |
| `strings_Replace(s string, old string, new string, n int)` | Replace first `n` occurrences of `old` with `new`. Use `-1` for all | `{{ strings_Replace "hello" "l" "L" -1 }}` → `heLLo`      |
| `strings_ToDate(date string)`                              | Parse date string (YYYY-MM-DD format)                               | `{{ strings_ToDate "2024-01-15" }}`                       |
| `strings_First(s string, n int)`                           | Get first `n` characters                                            | `{{ strings_First "hello" 2 }}` → `he`                    |
| `strings_Last(s string, n int)`                            | Get last `n` characters                                             | `{{ strings_Last "hello" 2 }}` → `lo`                     |
| `strings_Truncate(s string, n int)`                        | Truncate to `n` characters with ellipsis                            | `{{ strings_Truncate "hello world" 5 }}` → `hello...`     |
| `strings_TrimPrefix(s string, prefix string)`              | Remove prefix from string                                           | `{{ strings_TrimPrefix "hello" "he" }}` → `llo`           |
| `strings_TrimSuffix(s string, suffix string)`              | Remove suffix from string                                           | `{{ strings_TrimSuffix "hello" "lo" }}` → `hel`           |
| `strings_Join(sep string, values ...string)`               | Join strings with separator                                         | `{{ strings_Join "-" "a" "b" "c" }}` → `a-b-c`            |

### Collection Functions

| Function                      | Description                                   | Example                                      |
| ----------------------------- | --------------------------------------------- | -------------------------------------------- |
| `dict_Str(pairs ...string)`   | Create string dictionary from key-value pairs | `{{ dict_Str "key1" "val1" "key2" "val2" }}` |
| `slice_Str(values ...string)` | Create string slice                           | `{{ slice_Str "a" "b" "c" }}`                |
| `slice_Int(values ...int)`    | Create int slice                              | `{{ slice_Int 1 2 3 }}`                      |
| `slice_Uint(values ...uint)`  | Create uint slice                             | `{{ slice_Uint 1 2 3 }}`                     |

### Body Functions

| Function                         | Description                                                                                                                                                                                                 | Example                                                             |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| `body_FormData(pairs ...string)` | Create multipart form data from key-value pairs. Automatically sets the `Content-Type` header. Values starting with `@` are treated as file references (local path or URL). Use `@@` to escape literal `@`. | `{{ body_FormData "field1" "value1" "file" "@/path/to/file.pdf" }}` |

**`body_FormData` Details:**

```yaml
# Text fields only
body: '{{ body_FormData "username" "john" "email" "john@example.com" }}'

# Single file upload
body: '{{ body_FormData "document" "@/path/to/file.pdf" }}'

# File from URL
body: '{{ body_FormData "image" "@https://example.com/photo.jpg" }}'

# Mixed text fields and files
body: |
  {{ body_FormData
     "title" "My Report"
     "author" "John Doe"
     "cover" "@/path/to/cover.jpg"
     "document" "@/path/to/report.pdf"
  }}

# Multiple files with same field name
body: |
  {{ body_FormData
     "files" "@/path/to/file1.pdf"
     "files" "@/path/to/file2.pdf"
  }}

# Escape @ for literal value (sends "@username")
body: '{{ body_FormData "twitter" "@@username" }}'
```

> **Note:** Files are cached in memory after the first read. Subsequent requests reuse the cached content, avoiding repeated disk/network I/O.

### File Functions

| Function                     | Description                                                                                               | Example                                 |
| ---------------------------- | --------------------------------------------------------------------------------------------------------- | --------------------------------------- |
| `file_Base64(source string)` | Read a file (local path or URL) and return its Base64 encoded content. Files are cached after first read. | `{{ file_Base64 "/path/to/file.pdf" }}` |

**`file_Base64` Details:**

```yaml
# Local file as Base64 in JSON body
body: '{"file": "{{ file_Base64 "/path/to/document.pdf" }}", "filename": "document.pdf"}'

# Remote file as Base64
body: '{"image": "{{ file_Base64 "https://example.com/photo.jpg" }}"}'

# Combined with values for reuse
values: "FILE_DATA={{ file_Base64 \"/path/to/file.bin\" }}"
body: '{"data": "{{ .Values.FILE_DATA }}"}'
```

## Fake Data Functions

These functions are powered by [gofakeit](https://github.com/brianvoe/gofakeit) library.

### File

| Function               | Description    | Example Output       |
| ---------------------- | -------------- | -------------------- |
| `fakeit_FileExtension` | File extension | `"nes"`              |
| `fakeit_FileMimeType`  | MIME type      | `"application/json"` |

### ID

| Function      | Description                       | Example Output                           |
| ------------- | --------------------------------- | ---------------------------------------- |
| `fakeit_ID`   | Generate random unique identifier | `"pfsfktb87rcmj6bqha2fz9"`               |
| `fakeit_UUID` | Generate UUID v4                  | `"b4ddf623-4ea6-48e5-9292-541f028d1fdb"` |

### Product

| Function                    | Description         | Example Output                    |
| --------------------------- | ------------------- | --------------------------------- |
| `fakeit_ProductName`        | Product name        | `"olive copper monitor"`          |
| `fakeit_ProductDescription` | Product description | `"Backwards caused quarterly..."` |
| `fakeit_ProductCategory`    | Product category    | `"clothing"`                      |
| `fakeit_ProductFeature`     | Product feature     | `"ultra-lightweight"`             |
| `fakeit_ProductMaterial`    | Product material    | `"brass"`                         |
| `fakeit_ProductUPC`         | UPC code            | `"012780949980"`                  |
| `fakeit_ProductAudience`    | Target audience     | `["adults"]`                      |
| `fakeit_ProductDimension`   | Product dimension   | `"medium"`                        |
| `fakeit_ProductUseCase`     | Use case            | `"home"`                          |
| `fakeit_ProductBenefit`     | Product benefit     | `"comfort"`                       |
| `fakeit_ProductSuffix`      | Product suffix      | `"pro"`                           |
| `fakeit_ProductISBN`        | ISBN number         | `"978-1-4028-9462-6"`             |

### Person

| Function                | Description            | Example Output           |
| ----------------------- | ---------------------- | ------------------------ |
| `fakeit_Name`           | Full name              | `"Markus Moen"`          |
| `fakeit_NamePrefix`     | Name prefix            | `"Mr."`                  |
| `fakeit_NameSuffix`     | Name suffix            | `"Jr."`                  |
| `fakeit_FirstName`      | First name             | `"Markus"`               |
| `fakeit_MiddleName`     | Middle name            | `"Belinda"`              |
| `fakeit_LastName`       | Last name              | `"Daniel"`               |
| `fakeit_Gender`         | Gender                 | `"male"`                 |
| `fakeit_Age`            | Age                    | `40`                     |
| `fakeit_Ethnicity`      | Ethnicity              | `"German"`               |
| `fakeit_SSN`            | Social Security Number | `"296446360"`            |
| `fakeit_EIN`            | Employer ID Number     | `"12-3456789"`           |
| `fakeit_Hobby`          | Hobby                  | `"Swimming"`             |
| `fakeit_Email`          | Email address          | `"markusmoen@pagac.net"` |
| `fakeit_Phone`          | Phone number           | `"6136459948"`           |
| `fakeit_PhoneFormatted` | Formatted phone        | `"136-459-9489"`         |

### Generate

| Function                       | Description                            | Example                                                |
| ------------------------------ | -------------------------------------- | ------------------------------------------------------ |
| `fakeit_Regex(pattern string)` | Generate string matching regex pattern | `{{ fakeit_Regex "[a-z]{5}[0-9]{3}" }}` → `"abcde123"` |

### Auth

| Function                                                                                      | Description                                                 | Example                                               |
| --------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------- |
| `fakeit_Username`                                                                             | Username                                                    | `"Daniel1364"`                                        |
| `fakeit_Password(upper bool, lower bool, numeric bool, special bool, space bool, length int)` | Generate password with specified character types and length | `{{ fakeit_Password true true true false false 16 }}` |

### Address

| Function                                            | Description                  | Example Output                                      |
| --------------------------------------------------- | ---------------------------- | --------------------------------------------------- |
| `fakeit_City`                                       | City name                    | `"Marcelside"`                                      |
| `fakeit_Country`                                    | Country name                 | `"United States of America"`                        |
| `fakeit_CountryAbr`                                 | Country abbreviation         | `"US"`                                              |
| `fakeit_State`                                      | State name                   | `"Illinois"`                                        |
| `fakeit_StateAbr`                                   | State abbreviation           | `"IL"`                                              |
| `fakeit_Street`                                     | Full street                  | `"364 East Rapidsborough"`                          |
| `fakeit_StreetName`                                 | Street name                  | `"View"`                                            |
| `fakeit_StreetNumber`                               | Street number                | `"13645"`                                           |
| `fakeit_StreetPrefix`                               | Street prefix                | `"East"`                                            |
| `fakeit_StreetSuffix`                               | Street suffix                | `"Ave"`                                             |
| `fakeit_Unit`                                       | Unit                         | `"Apt 123"`                                         |
| `fakeit_Zip`                                        | ZIP code                     | `"13645"`                                           |
| `fakeit_Latitude`                                   | Random latitude              | `-73.534056`                                        |
| `fakeit_Longitude`                                  | Random longitude             | `-147.068112`                                       |
| `fakeit_LatitudeInRange(min float64, max float64)`  | Latitude in specified range  | `{{ fakeit_LatitudeInRange 0 90 }}` → `22.921026`   |
| `fakeit_LongitudeInRange(min float64, max float64)` | Longitude in specified range | `{{ fakeit_LongitudeInRange 0 180 }}` → `-8.170450` |

### Game

| Function          | Description | Example Output      |
| ----------------- | ----------- | ------------------- |
| `fakeit_Gamertag` | Gamer tag   | `"footinterpret63"` |

### Beer

| Function             | Description     | Example Output                |
| -------------------- | --------------- | ----------------------------- |
| `fakeit_BeerAlcohol` | Alcohol content | `"2.7%"`                      |
| `fakeit_BeerBlg`     | Blg             | `"6.4°Blg"`                   |
| `fakeit_BeerHop`     | Hop             | `"Glacier"`                   |
| `fakeit_BeerIbu`     | IBU             | `"29 IBU"`                    |
| `fakeit_BeerMalt`    | Malt            | `"Munich"`                    |
| `fakeit_BeerName`    | Beer name       | `"Duvel"`                     |
| `fakeit_BeerStyle`   | Beer style      | `"European Amber Lager"`      |
| `fakeit_BeerYeast`   | Yeast           | `"1388 - Belgian Strong Ale"` |

### Car

| Function                     | Description  | Example Output         |
| ---------------------------- | ------------ | ---------------------- |
| `fakeit_CarMaker`            | Car maker    | `"Nissan"`             |
| `fakeit_CarModel`            | Car model    | `"Aveo"`               |
| `fakeit_CarType`             | Car type     | `"Passenger car mini"` |
| `fakeit_CarFuelType`         | Fuel type    | `"CNG"`                |
| `fakeit_CarTransmissionType` | Transmission | `"Manual"`             |

### Words

| Function                           | Description                 | Example Output   |
| ---------------------------------- | --------------------------- | ---------------- |
| `fakeit_Word`                      | Random word                 | `"example"`      |
| `fakeit_Noun`                      | Random noun                 | `"computer"`     |
| `fakeit_NounCommon`                | Common noun                 | `"table"`        |
| `fakeit_NounConcrete`              | Concrete noun               | `"chair"`        |
| `fakeit_NounAbstract`              | Abstract noun               | `"freedom"`      |
| `fakeit_NounCollectivePeople`      | Collective noun (people)    | `"team"`         |
| `fakeit_NounCollectiveAnimal`      | Collective noun (animal)    | `"herd"`         |
| `fakeit_NounCollectiveThing`       | Collective noun (thing)     | `"bunch"`        |
| `fakeit_NounCountable`             | Countable noun              | `"book"`         |
| `fakeit_NounUncountable`           | Uncountable noun            | `"water"`        |
| `fakeit_Verb`                      | Random verb                 | `"run"`          |
| `fakeit_VerbAction`                | Action verb                 | `"jump"`         |
| `fakeit_VerbLinking`               | Linking verb                | `"is"`           |
| `fakeit_VerbHelping`               | Helping verb                | `"can"`          |
| `fakeit_Adverb`                    | Random adverb               | `"quickly"`      |
| `fakeit_AdverbManner`              | Manner adverb               | `"carefully"`    |
| `fakeit_AdverbDegree`              | Degree adverb               | `"very"`         |
| `fakeit_AdverbPlace`               | Place adverb                | `"here"`         |
| `fakeit_AdverbTimeDefinite`        | Definite time adverb        | `"yesterday"`    |
| `fakeit_AdverbTimeIndefinite`      | Indefinite time adverb      | `"soon"`         |
| `fakeit_AdverbFrequencyDefinite`   | Definite frequency adverb   | `"daily"`        |
| `fakeit_AdverbFrequencyIndefinite` | Indefinite frequency adverb | `"often"`        |
| `fakeit_Preposition`               | Random preposition          | `"on"`           |
| `fakeit_PrepositionSimple`         | Simple preposition          | `"in"`           |
| `fakeit_PrepositionDouble`         | Double preposition          | `"out of"`       |
| `fakeit_PrepositionCompound`       | Compound preposition        | `"according to"` |
| `fakeit_Adjective`                 | Random adjective            | `"beautiful"`    |
| `fakeit_AdjectiveDescriptive`      | Descriptive adjective       | `"large"`        |
| `fakeit_AdjectiveQuantitative`     | Quantitative adjective      | `"many"`         |
| `fakeit_AdjectiveProper`           | Proper adjective            | `"American"`     |
| `fakeit_AdjectiveDemonstrative`    | Demonstrative adjective     | `"this"`         |
| `fakeit_AdjectivePossessive`       | Possessive adjective        | `"my"`           |
| `fakeit_AdjectiveInterrogative`    | Interrogative adjective     | `"which"`        |
| `fakeit_AdjectiveIndefinite`       | Indefinite adjective        | `"some"`         |
| `fakeit_Pronoun`                   | Random pronoun              | `"he"`           |
| `fakeit_PronounPersonal`           | Personal pronoun            | `"I"`            |
| `fakeit_PronounObject`             | Object pronoun              | `"him"`          |
| `fakeit_PronounPossessive`         | Possessive pronoun          | `"mine"`         |
| `fakeit_PronounReflective`         | Reflective pronoun          | `"myself"`       |
| `fakeit_PronounDemonstrative`      | Demonstrative pronoun       | `"that"`         |
| `fakeit_PronounInterrogative`      | Interrogative pronoun       | `"who"`          |
| `fakeit_PronounRelative`           | Relative pronoun            | `"which"`        |
| `fakeit_Connective`                | Random connective           | `"however"`      |
| `fakeit_ConnectiveTime`            | Time connective             | `"then"`         |
| `fakeit_ConnectiveComparative`     | Comparative connective      | `"similarly"`    |
| `fakeit_ConnectiveComplaint`       | Complaint connective        | `"although"`     |
| `fakeit_ConnectiveListing`         | Listing connective          | `"firstly"`      |
| `fakeit_ConnectiveCasual`          | Casual connective           | `"because"`      |
| `fakeit_ConnectiveExamplify`       | Examplify connective        | `"for example"`  |

### Text

| Function                                                                                 | Description                                     | Example                                       |
| ---------------------------------------------------------------------------------------- | ----------------------------------------------- | --------------------------------------------- |
| `fakeit_Sentence`                                                                        | Random sentence                                 | `{{ fakeit_Sentence }}`                       |
| `fakeit_Paragraph`                                                                       | Random paragraph                                | `{{ fakeit_Paragraph }}`                      |
| `fakeit_LoremIpsumWord`                                                                  | Lorem ipsum word                                | `"lorem"`                                     |
| `fakeit_LoremIpsumSentence(wordCount int)`                                               | Lorem ipsum sentence with specified word count  | `{{ fakeit_LoremIpsumSentence 5 }}`           |
| `fakeit_LoremIpsumParagraph(paragraphs int, sentences int, words int, separator string)` | Lorem ipsum paragraphs with specified structure | `{{ fakeit_LoremIpsumParagraph 1 3 5 "\n" }}` |
| `fakeit_Question`                                                                        | Random question                                 | `"What is your name?"`                        |
| `fakeit_Quote`                                                                           | Random quote                                    | `"Life is what happens..."`                   |
| `fakeit_Phrase`                                                                          | Random phrase                                   | `"a piece of cake"`                           |

### Foods

| Function           | Description    | Example Output                           |
| ------------------ | -------------- | ---------------------------------------- |
| `fakeit_Fruit`     | Fruit          | `"Peach"`                                |
| `fakeit_Vegetable` | Vegetable      | `"Amaranth Leaves"`                      |
| `fakeit_Breakfast` | Breakfast food | `"Blueberry banana happy face pancakes"` |
| `fakeit_Lunch`     | Lunch food     | `"No bake hersheys bar pie"`             |
| `fakeit_Dinner`    | Dinner food    | `"Wild addicting dip"`                   |
| `fakeit_Snack`     | Snack          | `"Trail mix"`                            |
| `fakeit_Dessert`   | Dessert        | `"French napoleons"`                     |

### Misc

| Function           | Description    | Example Output |
| ------------------ | -------------- | -------------- |
| `fakeit_Bool`      | Random boolean | `true`         |
| `fakeit_FlipACoin` | Flip a coin    | `"Heads"`      |

### Color

| Function            | Description        | Example Output                                            |
| ------------------- | ------------------ | --------------------------------------------------------- |
| `fakeit_Color`      | Color name         | `"MediumOrchid"`                                          |
| `fakeit_HexColor`   | Hex color          | `"#a99fb4"`                                               |
| `fakeit_RGBColor`   | RGB color          | `[85, 224, 195]`                                          |
| `fakeit_SafeColor`  | Safe color         | `"black"`                                                 |
| `fakeit_NiceColors` | Nice color palette | `["#cfffdd", "#b4dec1", "#5c5863", "#a85163", "#ff1f4c"]` |

### Image

| Function                                  | Description               | Example                          |
| ----------------------------------------- | ------------------------- | -------------------------------- |
| `fakeit_ImageJpeg(width int, height int)` | Generate JPEG image bytes | `{{ fakeit_ImageJpeg 100 100 }}` |
| `fakeit_ImagePng(width int, height int)`  | Generate PNG image bytes  | `{{ fakeit_ImagePng 100 100 }}`  |

### Internet

| Function                          | Description                                | Example Output                                        |
| --------------------------------- | ------------------------------------------ | ----------------------------------------------------- |
| `fakeit_URL`                      | Random URL                                 | `"http://www.principalproductize.biz/target"`         |
| `fakeit_UrlSlug(words int)`       | URL slug with specified word count         | `{{ fakeit_UrlSlug 3 }}` → `"bathe-regularly-quiver"` |
| `fakeit_DomainName`               | Domain name                                | `"centraltarget.biz"`                                 |
| `fakeit_DomainSuffix`             | Domain suffix                              | `"org"`                                               |
| `fakeit_IPv4Address`              | IPv4 address                               | `"222.83.191.222"`                                    |
| `fakeit_IPv6Address`              | IPv6 address                               | `"2001:cafe:8898:ee17:bc35:9064:5866:d019"`           |
| `fakeit_MacAddress`               | MAC address                                | `"cb:ce:06:94:22:e9"`                                 |
| `fakeit_HTTPStatusCode`           | HTTP status code                           | `200`                                                 |
| `fakeit_HTTPStatusCodeSimple`     | Simple status code                         | `404`                                                 |
| `fakeit_LogLevel(logType string)` | Log level (types: general, syslog, apache) | `{{ fakeit_LogLevel "general" }}` → `"error"`         |
| `fakeit_HTTPMethod`               | HTTP method                                | `"HEAD"`                                              |
| `fakeit_HTTPVersion`              | HTTP version                               | `"HTTP/1.1"`                                          |
| `fakeit_UserAgent`                | Random User-Agent                          | `"Mozilla/5.0..."`                                    |
| `fakeit_ChromeUserAgent`          | Chrome User-Agent                          | `"Mozilla/5.0 (X11; Linux i686)..."`                  |
| `fakeit_FirefoxUserAgent`         | Firefox User-Agent                         | `"Mozilla/5.0 (Macintosh; U;..."`                     |
| `fakeit_OperaUserAgent`           | Opera User-Agent                           | `"Opera/8.39..."`                                     |
| `fakeit_SafariUserAgent`          | Safari User-Agent                          | `"Mozilla/5.0 (iPad;..."`                             |
| `fakeit_APIUserAgent`             | API User-Agent                             | `"curl/8.2.5"`                                        |

### HTML

| Function           | Description     | Example Output     |
| ------------------ | --------------- | ------------------ |
| `fakeit_InputName` | HTML input name | `"email"`          |
| `fakeit_Svg`       | SVG image       | `"<svg>...</svg>"` |

### Date/Time

| Function                                           | Description                       | Example                                                                              |
| -------------------------------------------------- | --------------------------------- | ------------------------------------------------------------------------------------ |
| `fakeit_Date`                                      | Random date                       | `2023-06-15 14:30:00`                                                                |
| `fakeit_PastDate`                                  | Past date                         | `2022-03-10 09:15:00`                                                                |
| `fakeit_FutureDate`                                | Future date                       | `2025-12-20 18:45:00`                                                                |
| `fakeit_DateRange(start time.Time, end time.Time)` | Random date between start and end | `{{ fakeit_DateRange (strings_ToDate "2020-01-01") (strings_ToDate "2025-12-31") }}` |
| `fakeit_NanoSecond`                                | Nanosecond                        | `123456789`                                                                          |
| `fakeit_Second`                                    | Second (0-59)                     | `45`                                                                                 |
| `fakeit_Minute`                                    | Minute (0-59)                     | `30`                                                                                 |
| `fakeit_Hour`                                      | Hour (0-23)                       | `14`                                                                                 |
| `fakeit_Month`                                     | Month (1-12)                      | `6`                                                                                  |
| `fakeit_MonthString`                               | Month name                        | `"June"`                                                                             |
| `fakeit_Day`                                       | Day (1-31)                        | `15`                                                                                 |
| `fakeit_WeekDay`                                   | Weekday                           | `"Monday"`                                                                           |
| `fakeit_Year`                                      | Year                              | `2024`                                                                               |
| `fakeit_TimeZone`                                  | Timezone                          | `"America/New_York"`                                                                 |
| `fakeit_TimeZoneAbv`                               | Timezone abbreviation             | `"EST"`                                                                              |
| `fakeit_TimeZoneFull`                              | Full timezone                     | `"Eastern Standard Time"`                                                            |
| `fakeit_TimeZoneOffset`                            | Timezone offset                   | `-5`                                                                                 |
| `fakeit_TimeZoneRegion`                            | Timezone region                   | `"America"`                                                                          |

### Payment

| Function                                 | Description                                           | Example                                                        |
| ---------------------------------------- | ----------------------------------------------------- | -------------------------------------------------------------- |
| `fakeit_Price(min float64, max float64)` | Random price in range                                 | `{{ fakeit_Price 1 100 }}` → `92.26`                           |
| `fakeit_CreditCardCvv`                   | CVV                                                   | `"513"`                                                        |
| `fakeit_CreditCardExp`                   | Expiration date                                       | `"01/27"`                                                      |
| `fakeit_CreditCardNumber(gaps bool)`     | Credit card number. `gaps`: add spaces between groups | `{{ fakeit_CreditCardNumber true }}` → `"4111 1111 1111 1111"` |
| `fakeit_CreditCardType`                  | Card type                                             | `"Visa"`                                                       |
| `fakeit_CurrencyLong`                    | Currency name                                         | `"United States Dollar"`                                       |
| `fakeit_CurrencyShort`                   | Currency code                                         | `"USD"`                                                        |
| `fakeit_AchRouting`                      | ACH routing number                                    | `"513715684"`                                                  |
| `fakeit_AchAccount`                      | ACH account number                                    | `"491527954328"`                                               |
| `fakeit_BitcoinAddress`                  | Bitcoin address                                       | `"1BoatSLRHtKNngkdXEeobR76b53LETtpyT"`                         |
| `fakeit_BitcoinPrivateKey`               | Bitcoin private key                                   | `"5HueCGU8rMjxEXxiPuD5BDuG6o5xjA7QkbPp"`                       |
| `fakeit_BankName`                        | Bank name                                             | `"Wells Fargo"`                                                |
| `fakeit_BankType`                        | Bank type                                             | `"Investment Bank"`                                            |

### Finance

| Function       | Description      | Example Output   |
| -------------- | ---------------- | ---------------- |
| `fakeit_Cusip` | CUSIP identifier | `"38259P508"`    |
| `fakeit_Isin`  | ISIN identifier  | `"US38259P5089"` |

### Company

| Function               | Description    | Example Output                             |
| ---------------------- | -------------- | ------------------------------------------ |
| `fakeit_BS`            | Business speak | `"front-end"`                              |
| `fakeit_Blurb`         | Company blurb  | `"word"`                                   |
| `fakeit_BuzzWord`      | Buzzword       | `"disintermediate"`                        |
| `fakeit_Company`       | Company name   | `"Moen, Pagac and Wuckert"`                |
| `fakeit_CompanySuffix` | Company suffix | `"Inc"`                                    |
| `fakeit_JobDescriptor` | Job descriptor | `"Central"`                                |
| `fakeit_JobLevel`      | Job level      | `"Assurance"`                              |
| `fakeit_JobTitle`      | Job title      | `"Director"`                               |
| `fakeit_Slogan`        | Company slogan | `"Universal seamless Focus, interactive."` |

### Hacker

| Function                    | Description         | Example Output                                                                                |
| --------------------------- | ------------------- | --------------------------------------------------------------------------------------------- |
| `fakeit_HackerAbbreviation` | Hacker abbreviation | `"ADP"`                                                                                       |
| `fakeit_HackerAdjective`    | Hacker adjective    | `"wireless"`                                                                                  |
| `fakeit_HackeringVerb`      | Hackering verb      | `"connecting"`                                                                                |
| `fakeit_HackerNoun`         | Hacker noun         | `"driver"`                                                                                    |
| `fakeit_HackerPhrase`       | Hacker phrase       | `"If we calculate the program, we can get to the AI pixel through the redundant XSS matrix!"` |
| `fakeit_HackerVerb`         | Hacker verb         | `"synthesize"`                                                                                |

### Hipster

| Function                  | Description       | Example                                                             |
| ------------------------- | ----------------- | ------------------------------------------------------------------- |
| `fakeit_HipsterWord`      | Hipster word      | `"microdosing"`                                                     |
| `fakeit_HipsterSentence`  | Hipster sentence  | `"Soul loops with you probably haven't heard of them undertones."`  |
| `fakeit_HipsterParagraph` | Hipster paragraph | `"Single-origin austin, double why. Tag it Yuccie, keep it any..."` |

### App

| Function            | Description | Example Output        |
| ------------------- | ----------- | --------------------- |
| `fakeit_AppName`    | App name    | `"Parkrespond"`       |
| `fakeit_AppVersion` | App version | `"1.12.14"`           |
| `fakeit_AppAuthor`  | App author  | `"Qado Energy, Inc."` |

### Animal

| Function            | Description | Example Output      |
| ------------------- | ----------- | ------------------- |
| `fakeit_PetName`    | Pet name    | `"Ozzy Pawsborne"`  |
| `fakeit_Animal`     | Animal      | `"elk"`             |
| `fakeit_AnimalType` | Animal type | `"amphibians"`      |
| `fakeit_FarmAnimal` | Farm animal | `"Chicken"`         |
| `fakeit_Cat`        | Cat breed   | `"Chausie"`         |
| `fakeit_Dog`        | Dog breed   | `"Norwich Terrier"` |
| `fakeit_Bird`       | Bird        | `"goose"`           |

### Emoji

| Function                  | Description                                    | Example Output                                         |
| ------------------------- | ---------------------------------------------- | ------------------------------------------------------ |
| `fakeit_Emoji`            | Random emoji                                   | `"🤣"`                                                 |
| `fakeit_EmojiCategory`    | Emoji category                                 | `"Smileys & Emotion"`                                  |
| `fakeit_EmojiAlias`       | Emoji alias                                    | `"smile"`                                              |
| `fakeit_EmojiTag`         | Emoji tag                                      | `"happy"`                                              |
| `fakeit_EmojiFlag`        | Flag emoji                                     | `"🇺🇸"`                                                 |
| `fakeit_EmojiAnimal`      | Animal emoji                                   | `"🐱"`                                                 |
| `fakeit_EmojiFood`        | Food emoji                                     | `"🍕"`                                                 |
| `fakeit_EmojiPlant`       | Plant emoji                                    | `"🌸"`                                                 |
| `fakeit_EmojiMusic`       | Music emoji                                    | `"🎵"`                                                 |
| `fakeit_EmojiVehicle`     | Vehicle emoji                                  | `"🚗"`                                                 |
| `fakeit_EmojiSport`       | Sport emoji                                    | `"⚽"`                                                 |
| `fakeit_EmojiFace`        | Face emoji                                     | `"😊"`                                                 |
| `fakeit_EmojiHand`        | Hand emoji                                     | `"👋"`                                                 |
| `fakeit_EmojiClothing`    | Clothing emoji                                 | `"👕"`                                                 |
| `fakeit_EmojiLandmark`    | Landmark emoji                                 | `"🗽"`                                                 |
| `fakeit_EmojiElectronics` | Electronics emoji                              | `"📱"`                                                 |
| `fakeit_EmojiGame`        | Game emoji                                     | `"🎮"`                                                 |
| `fakeit_EmojiTools`       | Tools emoji                                    | `"🔧"`                                                 |
| `fakeit_EmojiWeather`     | Weather emoji                                  | `"☀️"`                                                 |
| `fakeit_EmojiJob`         | Job emoji                                      | `"👨‍💻"`                                                 |
| `fakeit_EmojiPerson`      | Person emoji                                   | `"👤"`                                                 |
| `fakeit_EmojiGesture`     | Gesture emoji                                  | `"🙌"`                                                 |
| `fakeit_EmojiCostume`     | Costume emoji                                  | `"🎃"`                                                 |
| `fakeit_EmojiSentence`    | Emoji sentence with random emojis interspersed | `"Weekends reserve time for 🖼️ Disc 🏨 golf and day."` |

### Language

| Function                      | Description           | Example Output |
| ----------------------------- | --------------------- | -------------- |
| `fakeit_Language`             | Language              | `"English"`    |
| `fakeit_LanguageAbbreviation` | Language abbreviation | `"en"`         |
| `fakeit_ProgrammingLanguage`  | Programming language  | `"Go"`         |

### Number

| Function                                        | Description                         | Example                                    |
| ----------------------------------------------- | ----------------------------------- | ------------------------------------------ |
| `fakeit_Number(min int, max int)`               | Random number in range              | `{{ fakeit_Number 1 100 }}` → `42`         |
| `fakeit_Int`                                    | Random int                          | `{{ fakeit_Int }}`                         |
| `fakeit_IntN(n int)`                            | Random int from 0 to n              | `{{ fakeit_IntN 100 }}`                    |
| `fakeit_Int8`                                   | Random int8                         | `{{ fakeit_Int8 }}`                        |
| `fakeit_Int16`                                  | Random int16                        | `{{ fakeit_Int16 }}`                       |
| `fakeit_Int32`                                  | Random int32                        | `{{ fakeit_Int32 }}`                       |
| `fakeit_Int64`                                  | Random int64                        | `{{ fakeit_Int64 }}`                       |
| `fakeit_Uint`                                   | Random uint                         | `{{ fakeit_Uint }}`                        |
| `fakeit_UintN(n uint)`                          | Random uint from 0 to n             | `{{ fakeit_UintN 100 }}`                   |
| `fakeit_Uint8`                                  | Random uint8                        | `{{ fakeit_Uint8 }}`                       |
| `fakeit_Uint16`                                 | Random uint16                       | `{{ fakeit_Uint16 }}`                      |
| `fakeit_Uint32`                                 | Random uint32                       | `{{ fakeit_Uint32 }}`                      |
| `fakeit_Uint64`                                 | Random uint64                       | `{{ fakeit_Uint64 }}`                      |
| `fakeit_Float32`                                | Random float32                      | `{{ fakeit_Float32 }}`                     |
| `fakeit_Float32Range(min float32, max float32)` | Random float32 in range             | `{{ fakeit_Float32Range 0 100 }}`          |
| `fakeit_Float64`                                | Random float64                      | `{{ fakeit_Float64 }}`                     |
| `fakeit_Float64Range(min float64, max float64)` | Random float64 in range             | `{{ fakeit_Float64Range 0 100 }}`          |
| `fakeit_RandomInt(slice []int)`                 | Random int from slice               | `{{ fakeit_RandomInt (slice_Int 1 2 3) }}` |
| `fakeit_HexUint(bits int)`                      | Random hex uint with specified bits | `{{ fakeit_HexUint 8 }}` → `"0xff"`        |

### String

| Function                              | Description                     | Example                                                         |
| ------------------------------------- | ------------------------------- | --------------------------------------------------------------- |
| `fakeit_Digit`                        | Single random digit             | `"0"`                                                           |
| `fakeit_DigitN(n uint)`               | Generate `n` random digits      | `{{ fakeit_DigitN 5 }}` → `"71364"`                             |
| `fakeit_Letter`                       | Single random letter            | `"g"`                                                           |
| `fakeit_LetterN(n uint)`              | Generate `n` random letters     | `{{ fakeit_LetterN 10 }}` → `"gbRMaRxHki"`                      |
| `fakeit_Lexify(pattern string)`       | Replace `?` with random letters | `{{ fakeit_Lexify "?????@??????.com" }}` → `"billy@mister.com"` |
| `fakeit_Numerify(pattern string)`     | Replace `#` with random digits  | `{{ fakeit_Numerify "(###)###-####" }}` → `"(555)867-5309"`     |
| `fakeit_RandomString(slice []string)` | Random string from slice        | `{{ fakeit_RandomString (slice_Str "a" "b" "c") }}`             |

### Celebrity

| Function                   | Description        | Example Output     |
| -------------------------- | ------------------ | ------------------ |
| `fakeit_CelebrityActor`    | Celebrity actor    | `"Brad Pitt"`      |
| `fakeit_CelebrityBusiness` | Celebrity business | `"Elon Musk"`      |
| `fakeit_CelebritySport`    | Celebrity sport    | `"Michael Phelps"` |

### Minecraft

| Function                          | Description       | Example Output   |
| --------------------------------- | ----------------- | ---------------- |
| `fakeit_MinecraftOre`             | Minecraft ore     | `"coal"`         |
| `fakeit_MinecraftWood`            | Minecraft wood    | `"oak"`          |
| `fakeit_MinecraftArmorTier`       | Armor tier        | `"iron"`         |
| `fakeit_MinecraftArmorPart`       | Armor part        | `"helmet"`       |
| `fakeit_MinecraftWeapon`          | Minecraft weapon  | `"bow"`          |
| `fakeit_MinecraftTool`            | Minecraft tool    | `"shovel"`       |
| `fakeit_MinecraftDye`             | Minecraft dye     | `"white"`        |
| `fakeit_MinecraftFood`            | Minecraft food    | `"apple"`        |
| `fakeit_MinecraftAnimal`          | Minecraft animal  | `"chicken"`      |
| `fakeit_MinecraftVillagerJob`     | Villager job      | `"farmer"`       |
| `fakeit_MinecraftVillagerStation` | Villager station  | `"furnace"`      |
| `fakeit_MinecraftVillagerLevel`   | Villager level    | `"master"`       |
| `fakeit_MinecraftMobPassive`      | Passive mob       | `"cow"`          |
| `fakeit_MinecraftMobNeutral`      | Neutral mob       | `"bee"`          |
| `fakeit_MinecraftMobHostile`      | Hostile mob       | `"spider"`       |
| `fakeit_MinecraftMobBoss`         | Boss mob          | `"ender dragon"` |
| `fakeit_MinecraftBiome`           | Minecraft biome   | `"forest"`       |
| `fakeit_MinecraftWeather`         | Minecraft weather | `"rain"`         |

### Book

| Function            | Description | Example Output |
| ------------------- | ----------- | -------------- |
| `fakeit_BookTitle`  | Book title  | `"Hamlet"`     |
| `fakeit_BookAuthor` | Book author | `"Mark Twain"` |
| `fakeit_BookGenre`  | Book genre  | `"Adventure"`  |

### Movie

| Function            | Description | Example Output |
| ------------------- | ----------- | -------------- |
| `fakeit_MovieName`  | Movie name  | `"Inception"`  |
| `fakeit_MovieGenre` | Movie genre | `"Sci-Fi"`     |

### Error

| Function                 | Description       | Example Output                     |
| ------------------------ | ----------------- | ---------------------------------- |
| `fakeit_Error`           | Random error      | `"connection refused"`             |
| `fakeit_ErrorDatabase`   | Database error    | `"database connection failed"`     |
| `fakeit_ErrorGRPC`       | gRPC error        | `"rpc error: code = Unavailable"`  |
| `fakeit_ErrorHTTP`       | HTTP error        | `"HTTP 500 Internal Server Error"` |
| `fakeit_ErrorHTTPClient` | HTTP client error | `"HTTP 404 Not Found"`             |
| `fakeit_ErrorHTTPServer` | HTTP server error | `"HTTP 503 Service Unavailable"`   |
| `fakeit_ErrorRuntime`    | Runtime error     | `"panic: runtime error"`           |

### School

| Function        | Description | Example Output         |
| --------------- | ----------- | ---------------------- |
| `fakeit_School` | School name | `"Harvard University"` |

### Song

| Function            | Description | Example Output        |
| ------------------- | ----------- | --------------------- |
| `fakeit_SongName`   | Song name   | `"Bohemian Rhapsody"` |
| `fakeit_SongArtist` | Song artist | `"Queen"`             |
| `fakeit_SongGenre`  | Song genre  | `"Rock"`              |
