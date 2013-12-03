package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/codegangsta/martini"
	_ "github.com/lib/pq"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

type ImageStitch struct {
	Photos []Photo
	VideoDest string

	WorkDir string
}

func checkForTool(tool string) bool {
	_, err := exec.LookPath("mogrify")
	if err != nil {
		log.Fatal("%s is in not installed!", tool)
		return false
	}

	return true
}

// mkdir temp
// cp *.JPG temp/.
// mogrify -resize 800x800  temp/*.JPG
// convert temp/*.JPG -delay 10 -morph 10 temp/%05d.jpg
// ffmpeg -r 25 -qscale 2  -i temp/%05d.jpg output.mp4
// # rm -R temp

func (i *ImageStitch) ResizeImages(resize_size string) {
	if resize_size == "" {
		resize_size = "800x600"
	}

	files := fmt.Sprintf("%s/%s", i.WorkDir, "*.jpg")

	cmd := exec.Command("mogrify", "-resize", resize_size, files)

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ResizeImage: %q\n", out.String())
}

func (i *ImageStitch) MorphImages() {
	// convert *.JPG -delay 10 -morph 10 %05d.morph.jpg
	files := fmt.Sprintf("%s/%s", i.WorkDir, "*.jpg")

	cmd := exec.Command("convert", files, "-delay", "3", "-morph", "10", fmt.Sprintf("%s/%s", i.WorkDir, "%05d.morph.jpg"))

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("MorphImages %q\n", out.String()) 
}

func (i *ImageStitch) CreateVideo() {
	i.VideoDest = fmt.Sprintf("%s/%s", i.WorkDir, "imageskitch.mp4")

	//ffmpeg -r 25 -qscale 2 -i %05d.morph.jpg output.mp4
	cmd := exec.Command("ffmpeg", "-r", "25", "-qscale", "2", "-i", fmt.Sprintf("%s/%s", i.WorkDir, "%05d.morph.jpg"), i.VideoDest)

	var out bytes.Buffer
	cmd.Stdout = &out

		log.Printf(fmt.Sprintf("%s/%s", i.WorkDir, "%05d.morph.jpg"))
	log.Printf("asdsad")
		log.Printf(i.VideoDest)
	log.Printf("asdsad")

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("MorphImages %s: %q\n", i.VideoDest, out.String()) 
}

func NewImageStitch(photos []Photo) *ImageStitch {
	workdir, err := ioutil.TempDir("", "imagestitch")
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	}

	return &ImageStitch{
		Photos: photos,
		WorkDir:   workdir,
	}
}

type Photo struct {
	Url       string
	Timestamp string
}

func StitchWorker(user_id string) {
	log.Printf("Starting Stitch for %s", user_id)

	db := dbConnect()

	rows, err := db.Query("select photo_url, extract(epoch from created_at) from photos where user_id = $1", user_id)
	if err != nil {
		log.Fatal(err)
	}

	var photos []Photo
	for rows.Next() {
		var photo Photo
		rows.Scan(&photo.Url, &photo.Timestamp)

		photos = append(photos, photo)
	}

	image_stitch := NewImageStitch(photos)
	image_stitch.ResizeImages("")
	image_stitch.MorphImages()
	image_stitch.CreateVideo()

	log.Printf("Finished Stitch for %s", user_id)
}

func getParseUser(user_id string) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.parse.com/1/users/%s", user_id), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("X-Parse-Application-Id", "iX3szWRnSwOsy0ec0KGPWCUrbchGFhQ9ySrWfjPQ")
	req.Header.Add("X-Parse-REST-API-Key", "xj4sBbRSWY1naZpvbZCjQhPRI7E27JsN89Zs6MOU")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func PostStitchHandler(params martini.Params) string {
	//	go StitchWorker(params["user_id"])

	return params["user_id"]
}

func GetStitchHandler(params martini.Params) string {

	/*
	   curl -X GET \
	     -H "X-Parse-Application-Id: ur4XEx6YMhw0XOCiEC5A4lrgLy9LnY0rk6FdwpzE" \
	     -H "X-Parse-REST-API-Key: mbqgNhyjH1vcN1VA5xTQwEqKnQD5BcXQUvwubOvz" \
	     https://api.parse.com/1/classes/GameScore
	*/
	return "Hi"
}

func PostPhotoHandler(params martini.Params) string {

	return "Uploaded"
}

func dbConnect() *sql.DB {
	db, err := sql.Open("postgres", "dbname=stitchy sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func main() {
	checkForTool("mogrify")
	checkForTool("convert")
	checkForTool("ffmpeg")

	StitchWorker("1")

	//	getParseUser("YhCrKEIF4D")
	//	getParseFiles("YhCrKEIF4D")
	// m := martini.Classic()
	// m.Get("/v1/users/:user_id/stitch",  GetStitchHandler)
	// m.Post("/v1/users/:user_id/stitch", PostStitchHandler)
	//	m.Post("/v1/users/:user_id/photo", PostPhotoHandler)
	// m.Run()
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
