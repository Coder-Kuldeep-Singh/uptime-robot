package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/joho/godotenv"
)

type countdown struct {
	t int
	d int
	h int
	m int
	s int
}

// URL putting all urls' globally so we don't need to read file again read one time and
// process data until program exits
var (
	URL              []string
	Sent             = 0
	RunningFirstTime = 0
)

// Open opens the file and return back the file content or error
func Open(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Read method reads the txt file line by line and returns back the slice of arrays
func Read(w *os.File, wg *sync.WaitGroup) {
	empty := IsEmpty(w)
	if empty {
		os.Exit(1)
	}
	scanner := bufio.NewScanner(w)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if len(scanner.Text()) == 0 {
			continue
		}
		URL = append(URL, scanner.Text())
	}
	defer w.Close()
	wg.Done()
}

// IsEmpty checks the size of the file
func IsEmpty(w *os.File) bool {
	info, err := w.Stat()
	if err != nil {
		log.Printf("error to get file info %s", err)
		return true
	}
	if info.Size() == 0 {
		log.Println("empty file can't process")
		return true
	}
	return false
}

// createClient creates a custom client for this tool
func createClient() *http.Client {
	return &http.Client{Timeout: time.Second * 10,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errors.New("something bad happened") // or maybe the error from the request
		}}
}

// CreateRequest generates the request for the given url
func CreateRequest(method string, url string, body io.Reader) (req *http.Request, err error) {
	req, err = http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("Error reading request %s.", err.Error())
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:10.0) Gecko/20100101 Firefox/10.0")
	return
}

// request sends the request to the url with the help of client and clientRequest function
func request(url string) (resp *http.Response, err error) {
	client := createClient()
	req, err := CreateRequest(http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	return
}

func reset() {
	Sent = 0
}

// CronSentReset resets the Sent variable in every 15 minutes so we can avoid
// the multiple email sends
func CronSentReset() {
	schedule := gocron.NewScheduler(time.UTC)
	_, err := schedule.Every(15).Minute().Do(reset)
	if err != nil {
		log.Printf("email sent variable updates' cron func failed %s", err.Error())
		return
	}
	<-schedule.StartAsync()
	schedule.Stop()
}

// UptimeCron runs in every 5 minutes
func UptimeCron() {
	RunningFirstTime++
	schedule := gocron.NewScheduler(time.UTC)
	_, err := schedule.Every(5).Minute().Do(UptimeRobot)
	if err != nil {
		log.Printf("Uptime robots' cron func failed %s", err.Error())
		return
	}
	<-schedule.StartAsync()
	schedule.Stop()
}

// UptimeRobot checks the urls' status code
func UptimeRobot() {
	if len(URL) > 0 {
		req, err := request(URL[0])
		if err != nil {
			log.Printf("error while sending request %s", err.Error())
			return
		}
		if req.StatusCode > 399 {
			if len(URL) > 1 {
				log.Println("Processing the Other URL to Check Servers' Status")
				checkedMap, CheckedCount := ServerDown()
				if len(checkedMap) == CheckedCount-1 {
					log.Println("Checked All URL --> Server down")
					if Sent == 0 {
						message := os.Getenv("EMAILBODY")
						SendEmailAlert([]byte(message))
						Sent++
						log.Println("Email Service Disabled for 15 Minutes")
						return
					}
					return
				}
				log.Printf("Server is Running Perhaps there is the problem with %s url", URL[0])
				return
			}
		}
	}
}

// ServerDown checks more urls'
func ServerDown() (map[string]bool, int) {
	t := time.Now().Add(30 * time.Second)
	v, err := time.Parse(time.RFC3339, t.Format("2006-01-02T15:04:05Z07:00"))
	if err != nil {
		fmt.Println(err)
		return make(map[string]bool), 1
	}
	CrossCheck := make(map[string]bool)
	TotalChecked := 1
	for range time.Tick(1 * time.Second) {
		timeRemaining := getTimeRemaining(v)
		if timeRemaining.t <= 0 {
			fmt.Println("30 seconds extra check finished")
			break
		}
		if TotalChecked == len(URL) {
			break
		}
		for i := 1; i < len(URL); i++ {
			req, err := request(URL[i])
			if err != nil {
				log.Printf("error while sending  more request for checking request %s", err.Error())
				continue
			}
			if req.StatusCode > 399 {
				CrossCheck[URL[i]] = true
			}
			if len(URL) == i {
				break
			}
			TotalChecked++
		}
	}
	return CrossCheck, TotalChecked
}

// SendEmailAlert sends the alert email to owner||developer
func SendEmailAlert(body []byte) {
	from := configString("FROM")
	pass := configString("PASSWORD")
	to := configString("TO")
	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: Recommendation Service Failed\n\n" +
		string(body)
	hostPort := fmt.Sprintf("%s:%s", configString("SMTPHOST"), configString("SMTPPORT"))
	host := fmt.Sprintf("%s", configString("SMTPHOST"))
	err := smtp.SendMail(hostPort, smtp.PlainAuth("", from, pass, host), from, []string{to}, []byte(msg))
	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}
	log.Print("sent")
}

func configString(str string) string {
	return os.Getenv(str)
}

func getTimeRemaining(t time.Time) countdown {
	currentTime := time.Now()
	difference := t.Sub(currentTime)

	total := int(difference.Seconds())
	days := int(total / (60 * 60 * 24))
	hours := int(total / (60 * 60) % 24)
	minutes := int(total/60) % 60
	seconds := int(total % 60)

	return countdown{
		t: total,
		d: days,
		h: hours,
		m: minutes,
		s: seconds,
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Running"))
}

func main() {
	filePath := flag.String("f", "", "filename or you can provide the full path of the time so program can access that file for you")
	flag.Parse()
	if *filePath == "" {
		flag.Usage()
		return
	}
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file -> ", err)
		return
	}
	file, err := Open(*filePath)
	if err != nil {
		log.Printf("error to open file %s", err.Error())
		return
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go Read(file, &wg)
	wg.Wait()
	fmt.Println("Starts Running UPTIMEROBOT :)")
	// Running first time directly and then next run with cron
	UptimeRobot()
	go CronSentReset()
	go UptimeCron()

	http.HandleFunc("/", home)
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Printf("error to run servers %s", err.Error())
		return
	}
}
