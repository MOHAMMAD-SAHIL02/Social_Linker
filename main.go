package main
import  (
    "encoding/json"
    "fmt"
    "html/template"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strings"
)

type PageData struct {
    Links []string
}

type OpenAIResponse struct {
    Choices []struct {
        Text string `json:"text"`
    } `json:"choices"`
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/get_links", getLinksHandler)
    log.Println("Listening on PORT :8084")
    log.Fatal(http.ListenAndServe(":8084", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.ParseFiles("idx.html"))
    tmpl.Execute(w, nil)
}

func getLinksHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        domain := r.FormValue("domain")
        links, err := getSocialMediaLinks(domain)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        tmpl := template.Must(template.ParseFiles("idx.html"))
        data := PageData{Links: links}
        tmpl.Execute(w, data)
    } else {
        http.Redirect(w, r, "/", http.StatusSeeOther)
    }
}

func getSocialMediaLinks(domain string) ([]string, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Println("OpenAI API key not set")
        return nil, fmt.Errorf("OpenAI API key not set")
    }
    log.Println("OpenAI API key is set:", apiKey)

    prompt := fmt.Sprintf("Find the social media links for the domain %s", domain)
    requestBody, err := json.Marshal(map[string]interface{}{
        "model": "gpt-4", 
        "messages": []map[string]string{
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": prompt},
        },
        "max_tokens": 100,
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

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := ioutil.ReadAll(resp.Body)
        return nil, fmt.Errorf("failed to get response from OpenAI API, status code: %d, body: %s", resp.StatusCode, bodyBytes)
    }

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var openAIResp OpenAIResponse
    err = json.Unmarshal(body, &openAIResp)
    if err != nil {
        return nil, err
    }

    if len(openAIResp.Choices) == 0 {
        return nil, fmt.Errorf("no choices returned from OpenAI API")
    }

    responseText := openAIResp.Choices[0].Text
    links := extractLinks(responseText)

    return links, nil
}


func extractLinks(text string) []string {
    var links []string
    lines := strings.Split(text, "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
            links = append(links, line)
        }
    }
    return links
}