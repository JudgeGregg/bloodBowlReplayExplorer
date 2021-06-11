package main

import (
	"bytes"
	"embed"
	"encoding/xml"
	"github.com/pkg/browser"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

// content is our static web server content.
//go:embed static
var static embed.FS

func main() {
	go browser.OpenURL("http://localhost:8000")
	// Set routing rules
	http.HandleFunc("/", root)
	http.HandleFunc("/replay/", replay)
	fs := http.FileServer(http.FS(static))
	http.Handle("/static/", fs)

	//Use the default DefaultServeMux.
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func replay(w http.ResponseWriter, r *http.Request) {
	fileName := r.FormValue("replayFile")
	resp, err := postFile(fileName, "https://bloodbowl-parser.nw.r.appspot.com/upload")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
	w.Write(resp)
}

func postFile(fileName string, targetURL string) ([]byte, error) {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileName = fileName + ".bbrz"

	// this step is very important
	fileWriter, err := bodyWriter.CreateFormFile("file", fileName)
	if err != nil {
		log.Println("error writing to buffer")
		return nil, err
	}

	// open file handle
	fh, err := os.Open(fileName)
	if err != nil {
		log.Println("error opening file")
		log.Println(err)
		return nil, err
	}
	defer fh.Close()

	//iocopy
	_, err = io.Copy(fileWriter, fh)
	if err != nil {
		return nil, err
	}

	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(targetURL, contentType, bodyBuf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Println(resp.Status)
	return respBody, nil
}

func root(w http.ResponseWriter, _ *http.Request) {
	xmlFile, err := os.Open("ReplayIndex.xml")
	if err != nil {
		log.Println("No ReplayIndex file found, exiting")
		os.Exit(1)
	}
	defer xmlFile.Close()
	byteValue, _ := ioutil.ReadAll(xmlFile)

	var replayIndex ReplayIndex
	xml.Unmarshal(byteValue, &replayIndex)

	// Append last games to common rows
	replayIndex.Matches.Records = append(replayIndex.Matches.Records, replayIndex.Matches.Matches...)

	// Filter non competition games
	filteredMatches := []RowMatchRecord{}

	for _, record := range replayIndex.Matches.Records {
		if record.CompetitionName == "" {
			continue
		}
		date := record.ReplayFileName
		record.MatchDate = strings.Split(date, "_")[1]
		filteredMatches = append(filteredMatches, record)
	}

	// Find most common coach name
	coachesHisto := make(map[string]int)

	for _, record := range filteredMatches {
		coachHomeName := record.CoachHomeName
		coachAwayName := record.CoachAwayName
		coachesHisto[coachAwayName] += 1
		coachesHisto[coachHomeName] += 1
		log.Print("Competition: " + record.CompetitionName + " ")
		log.Print("CoachHomeName: " + record.CoachHomeName + " ")
		log.Println("CoachAwayName: " + record.CoachAwayName)
	}

	var mostPrevalentCoach string
	i := 0
	for name, number := range coachesHisto {
		if number > i {
			mostPrevalentCoach = name
			i = number
		}

	}

	finalMatches := []RowMatchRecord{}
	for _, record := range filteredMatches {
		coachHomeName := record.CoachHomeName
		coachAwayName := record.CoachAwayName
		if coachAwayName == mostPrevalentCoach {
			record.Coach = coachHomeName
			record.OpponentTeam = record.TeamHomeName
			record.OwnTeam = record.TeamAwayName
			record.OpponentScore = record.HomeScore
			record.OwnScore = record.AwayScore
		} else {
			record.Coach = coachAwayName
			record.OpponentTeam = record.TeamAwayName
			record.OwnTeam = record.TeamHomeName
			record.OpponentScore = record.AwayScore
			record.OwnScore = record.HomeScore
		}
		finalMatches = append(finalMatches, record)
	}

	t := template.New("Content Template")
	t = template.Must(t.ParseFS(static, "static/base.tmpl"))
	t.ExecuteTemplate(w, "content", finalMatches)

}

type ReplayIndex struct {
	XMLName xml.Name `xml:"ReplayIndex.xml"`
	Matches Matches  `xml:"Matches"`
}

type Matches struct {
	Records []RowMatchRecord `xml:"RowMatchRecord"`
	Matches []RowMatchRecord `xml:"MatchRecord"`
}

type RowMatchRecord struct {
	MatchDate       string
	Coach           string
	OpponentTeam    string
	OpponentScore   string
	OwnTeam         string
	OwnScore        string
	ReplayFileName  string `xml:"ReplayFileName"`
	CoachHomeName   string `xml:"CoachHomeName"`
	CoachAwayName   string `xml:"CoachAwayName"`
	TeamHomeName    string `xml:"TeamHomeName"`
	TeamAwayName    string `xml:"TeamAwayName"`
	HomeScore       string `xml:"HomeScore"`
	AwayScore       string `xml:"AwayScore"`
	CompetitionName string `xml:"CompetitionName"`
}
