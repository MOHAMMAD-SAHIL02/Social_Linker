package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "html/template"
    "github.com/PuerkitoBio/goquery"
)

type PageData struct {
    Links []string
}

type OpenAIResponse struct {
    Choices []struct {
        Message struct {
            Content string `json:"content"`
        } `json:"message"`
    } `json:"choices"`
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/get_links", getLinksHandler)
    log.Println("Listening on PORT :8086")
    log.Fatal(http.ListenAndServe(":8086", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.ParseFiles("./idx.html"))
    tmpl.Execute(w, nil)
}

func getLinksHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        domain := r.FormValue("domain")
        formattedDomain := ensureProtocol(domain)

        links, err := getSocialMediaLinks(formattedDomain)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        log.Println(links)
        tmpl := template.Must(template.ParseFiles("./idx.html"))
        data := PageData{Links: links}
        tmpl.Execute(w, data)
    } else {
        http.Redirect(w, r, "/", http.StatusSeeOther)
    }
}

func ensureProtocol(url string) string {
    if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
        return "http://" + url
    }
    return url
}

func scrapeLinks(domain string) ([]string, error) {
    response, err := http.Get(domain)
    if err != nil {
        return nil, err
    }
    defer response.Body.Close()

    if response.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch the webpage, status code: %d", response.StatusCode)
    }

    doc, err := goquery.NewDocumentFromReader(response.Body)
    if err != nil {
        return nil, err
    }

    var links []string
    doc.Find("a").Each(func(i int, s *goquery.Selection) {
        link, exists := s.Attr("href")
        if exists && (strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://")) {
            links = append(links, link)
        }
    })

    return links, nil
}

func getSocialMediaLinks(domain string) ([]string, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")

    if apiKey == "" || apiKey == "sk-None-VlNzpseaI91CRXpZdbiST3BlbkFJ7UInijs8nUxEl1JldsMo" {
        return nil, fmt.Errorf("OpenAI API key not set or invalid")
    }

    links, err := scrapeLinks(domain)
    if err != nil {
        return nil, err
    }

    log.Println("Scraped Links: ", links)

    prompt := fmt.Sprintf("From the following links, identify only the social media links, and return them in JSON format as follows: {\"social_media_links\": [\"link1\", \"link2\", ...]}\n\nLinks:\n%s", strings.Join(links, "\n"))
    requestBody, err := json.Marshal(map[string]interface{}{
        "model": "gpt-4",
        "messages": []map[string]string{
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": prompt},
        },
        "max_tokens": 1024,
    })
    if err != nil {
        return nil, err
    }

    req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", strings.NewReader(string(requestBody)))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to get response from OpenAI API, status code: %d, body: %s", resp.StatusCode, body)
    }

    var openAIResp OpenAIResponse
    err = json.Unmarshal(body, &openAIResp)
    if err != nil {
        log.Printf("error parsing response body: %v\n", err)
        return nil, err
    }

    if len(openAIResp.Choices) == 0 {
        return nil, fmt.Errorf("no choices returned from OpenAI API")
    }

    responseContent := openAIResp.Choices[0].Message.Content
    log.Println("OpenAI API Response: ", responseContent)
    var responseObject struct {
        SocialMediaLinks []string `json:"social_media_links"`
    }
    err = json.Unmarshal([]byte(responseContent), &responseObject)
    if err != nil {
        log.Printf("error parsing JSON content: %v\n", err)
        return nil, err
    }

    return responseObject.SocialMediaLinks, nil
}


