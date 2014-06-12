package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"github.com/codegangsta/martini"
	_ "github.com/lib/pq"
	"html"
	"io"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"net/http"
	"os"
	"os/exec"
)

const (
	Started    = "started"
	InProgress = "in-progress"
	Finished   = "finished"
)

type StitchStatus map[string]string

var (
	StitchingStatus StitchStatus
	bucketName      string
)

func NewStitchStatus() StitchStatus {
	return make(StitchStatus)
}

type ImageStitch struct {
	UserId string

	Photos    []Photo
	VideoDest string

	WorkDir string
}

func checkForTool(tool string) bool {
	_, err := exec.LookPath(tool)
	if err != nil {
		log.Println(fmt.Sprintf("%s is in not installed!", tool))
		return false
	}

	return true
}

func runCommand(command string, command_args []string) bytes.Buffer {
	cmd := exec.Command(command, command_args...)

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Println(err)
	}

	return out
}
func (i *ImageStitch) ResizeImages(resize_size string) {
	if resize_size == "" {
		resize_size = "800x600"
	}

	files := fmt.Sprintf("%s/%s", i.WorkDir, "*.jpg")
	command_args := []string{"-resize", resize_size, files}

	out := runCommand("mogrify", command_args)

	log.Printf("ResizeImage: %q\n", out.String())
}

func (i *ImageStitch) MorphImages() {
	files := fmt.Sprintf("%s/%s", i.WorkDir, "*.jpg")

	// convert *.jpg -delay 3 -morph 10 %05d.morph.jpg
	command_args := []string{files, "-limit", "memory", "100MB", "-delay", "3", "-morph", "10", fmt.Sprintf("%s/%s", i.WorkDir, "%05d.morph.jpg")}
	out := runCommand("convert", command_args)

	log.Printf("MorphImages %v %q\n", command_args, out.String())
}

func (i *ImageStitch) CreateVideo() {
	i.VideoDest = fmt.Sprintf("%s/%s", i.WorkDir, "imageskitch.mp4")

	// avconv -r 25 -qscale 2 -i %05d.morpg.jpg test.mp4
	command_args := []string{"-y", "-f", "image2", "-r", "25", "-qscale", "2", "-i", fmt.Sprintf("%s/%s", i.WorkDir, "%05d.morph.jpg"), i.VideoDest}
//	command_args := []string{"-r", "25", "-qscale", "2", "-i", fmt.Sprintf("%s/%s", i.WorkDir, "*.jpg"), i.VideoDest}
	out := runCommand("avconv", command_args)

	log.Printf("CreateVideo %v %s: %q\n", command_args, i.VideoDest, out.String())
}

func (i *ImageStitch) UploadVideo() {
	file, err := os.Open(i.VideoDest)
	if err != nil {
		log.Println(err)
	}

	data, err := ioutil.ReadAll(file)

	UploadToS3(fmt.Sprintf("%s/stitch.mp4", i.UserId), data, "video/mp4")
}

func UploadToS3(s3_dest string, data []byte, mime_type string) {
	// The AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables are used.
	auth, err := aws.EnvAuth()
	if err != nil {
		panic(err.Error())
	}

	// Open Bucket
	s := s3.New(auth, aws.USEast)
	bucket := s.Bucket(bucketName)

	fmt.Printf("buckname: %s", bucketName)

	err = bucket.Put(s3_dest, data, mime_type, s3.BucketOwnerFull)
	if err != nil {
		panic(err.Error())
	}
}

func (i *ImageStitch) RmWorkDir() {
	// TODO
}

// SendNotification to User

func NewImageStitch(user_id string, photos []Photo) *ImageStitch {
	workdir, err := ioutil.TempDir("", "imagestitch")
	if err != nil {
		log.Println(err)
	}
	log.Printf("WorkDir: %s", workdir)

	// Copy files to local.
	for _, photo := range photos {
		fmt.Println(photo.Timestamp)

		out, err := os.Create(fmt.Sprintf("%s/%s.jpg", workdir, photo.Timestamp))
		defer out.Close()

		resp, err := http.Get(photo.Url)
		defer resp.Body.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Println(err)
		}
	}

	return &ImageStitch{
		UserId:  user_id,
		Photos:  photos,
		WorkDir: workdir,
	}
}

type Photo struct {
	Url       string
	Timestamp string
}

func StitchWorker(user_id string) {
	log.Printf("Starting Stitch for %s", user_id)

	StitchingStatus[user_id] = Started

	// TODO: Make sure that the user exists before running this.

	db := dbConnect()

	rows, err := db.Query("select photo_url, extract(epoch from created_at) from photos where user_id = $1", user_id)
	if err != nil {
		log.Println(err)
	}

	var photos []Photo
	for rows.Next() {
		var photo Photo
		rows.Scan(&photo.Url, &photo.Timestamp)

		log.Printf("%s image", photo.Url)
		photos = append(photos, photo)
	}

	image_stitch := NewImageStitch(user_id, photos)
	image_stitch.ResizeImages("")
	image_stitch.MorphImages()
	image_stitch.CreateVideo()
	image_stitch.UploadVideo()
	image_stitch.RmWorkDir()

	StitchingStatus[user_id] = Finished

	log.Printf("Finished Stitch for %s", user_id)
}

func getParseUser(user_id string) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.parse.com/1/users/%s", user_id), nil)
	if err != nil {
		log.Println(err)
	}
	req.Header.Add("X-Parse-Application-Id", "iX3szWRnSwOsy0ec0KGPWCUrbchGFhQ9ySrWfjPQ")
	req.Header.Add("X-Parse-REST-API-Key", "xj4sBbRSWY1naZpvbZCjQhPRI7E27JsN89Zs6MOU")
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func PostStitchHandler(params martini.Params) string {
	user_id := params["user_id"]

	go StitchWorker(user_id)

	return StitchingStatus[user_id]
}

func GetStitchHandler(params martini.Params) string {
	user_id := params["user_id"]
	return StitchingStatus[user_id]
}

func PostPhotoHandler(w http.ResponseWriter, req *http.Request) {

	file, handler, err := req.FormFile("file")
	if err != nil {
		fmt.Println(err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}

	err = ioutil.WriteFile(handler.Filename, data, 0777)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("asd %s", handler.Filename)

	UploadToS3(fmt.Sprintf("1/%s", handler.Filename), data, "image/jpeg")
	// Update User#last_photo_at to be a timestamp.
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(req.URL.Path))
}

func dbConnect() *sql.DB {
	db, err := sql.Open("postgres", "dbname=stitchy sslmode=disable")
	if err != nil {
		log.Println(err)
	}

	return db
}

func NotificationWorker() {
	// Notification Handler which sends a notification to people
	// who haven't taken a picture in x time.
}

func main() {
	flag.StringVar(&bucketName, "b", "", "Bucket Name")
	flag.Parse()

	checkForTool("mogrify")
	checkForTool("convert")
	checkForTool("avconv")

	StitchingStatus = NewStitchStatus()

	go NotificationWorker()

	m := martini.Classic()
	m.Get("/v1/users/:user_id/stitch", GetStitchHandler)
	m.Post("/v1/users/:user_id/stitch", PostStitchHandler)
	m.Post("/v1/users/:user_id/photo", PostPhotoHandler)
	m.Run()
}

/*

 - APIS
   - POST /v1/users/:user_id/photo ?url=
   - POST /v1/users/:user_id/stitch
   - GET /v1/users/:user_id/stitch
     - Return { } - No Job
     - Return { "status": "in-progress", "" }
     - Return { "video_url": "" }

 - Client
    -> Talk to Parse for Login
    -> Take a Picture
       -> Upload to S3
       -> Update List of files for user
       -> Send to stitchy
    -> Client can query Parse or stitchy url to figure out status.


*/
