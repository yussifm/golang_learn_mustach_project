package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/golang/freetype/raster"
	"golang.org/x/image/math/fixed"
	"golang.org/x/oauth2"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)


var uploadTemplate = template.Must(template.ParseFiles("upload.html"))
var errorTemplate = template.Must(template.ParseFiles("error.html"))
var editTemplate = template.Must(template.ParseFiles("edit.html"))

// func handler(w http.ResponseWriter, r *http.Request) {

// 	uploadTemplate.Execute(w, nil)
// }

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func rgba(m image.Image) *image.RGBA {
	// Fast path: if m is already an RGBA, just return it.
	if r, ok := m.(*image.RGBA); ok {
		return r
	}
	// Create a new image and draw m into it.
	b := m.Bounds()
	r := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(r, b, m, image.ZP, draw.Over)
	return r
}

func edit(w http.ResponseWriter, r *http.Request) {
 editTemplate.Execute(w, r.FormValue("id"))
}


func moustache(m image.Image, x, y, size, droopFactor int) image.Image {
	mrgba := rgba(m)

	p := raster.NewRGBAPainter(mrgba)
	p.SetColor(image.Black.RGBA64At(0, 0))

	w, h := m.Bounds().Dx(), m.Bounds().Dy()

	// ...

	r := raster.NewRasterizer(w, h)

	var (
		mag = fixed.Int26_6.Ceil(fixed.Int26_6((10 + size) << 8))

		width = fixed.P(20, 0).Mul(fixed.Int26_6(mag))
		mid   = fixed.P(x, y)
		droop = fixed.P(0, int(fixed.Int26_6(droopFactor))).Mul(fixed.Int26_6(mag))
		left  = mid.Sub(width).Add(droop)
		right = mid.Add(width).Add(droop)
		bow   = fixed.P(0, 5).Mul(fixed.Int26_6(mag)).Sub(droop)
		curlx = fixed.P(10, 0).Mul(fixed.Int26_6(mag))
		curly = fixed.P(0, 2).Mul(fixed.Int26_6(mag))
		risex = fixed.P(2, 0).Mul(fixed.Int26_6(mag))
		risey = fixed.P(0, 5).Mul(fixed.Int26_6(mag))
	)
	r.Start(left)
	r.Add3(
		mid.Sub(curlx).Add(curly),
		mid.Sub(risex).Sub(risey),
		mid,
	)
	r.Add3(
		mid.Add(risex).Sub(risey),
		mid.Add(curlx).Add(curly),
		right,
	)
	r.Add2(
		mid.Add(bow),
		left,
	)
	r.Rasterize(p)
	return mrgba
}

func img(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
 key := datastore.NewKey("Image", r.FormValue("id"), 0, nil)
 im := new(Image)
 err := datastore.Get(c, key, im)
 check(err)
 i, _, err := image.Decode(bytes.NewBuffer(im.Data))
 check(err)

 f, err := os.Open("image-"+r.FormValue("id"))
 check(err)
 m, _, err := image.Decode(f)
 check(err)
 x, _ := strconv.Atoi(r.FormValue("x"))
 y, _ := strconv.Atoi(r.FormValue("y"))
 s, _ := strconv.Atoi(r.FormValue("s"))
 d, _ := strconv.Atoi(r.FormValue("d"))
 m = moustache(m, x, y, s, d)
 w.Header().Set("Content-type", "image/jpeg")
 jpeg.Encode(w, m, nil) // Default JPEG options.
}

func upload(w http.ResponseWriter, r *http.Request) {
 // ... same as before until we have the data in hand...
 // Grab the image data
 buf := new(bytes.Buffer)
 _, err = io.Copy(buf, f)
 check(err)
 // Create an App Engine context for the client's request.
 c := appengine.NewContext(r)
 // Save the image under a unique key, a hash of the image.
 key := datastore.NewKey("Image", keyOf(buf.Bytes()), 0, nil)
 _, err = datastore.Put(c, key, &Image{buf.Bytes()})
 check(err)
 // Redirect to /edit using the key.
 http.Redirect(w, r, "/edit?id="+key.StringID(),
 http.StatusFound)
}
func errorHandler(fn http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e, ok := recover().(error); ok {
				w.WriteHeader(http.StatusInternalServerError)
				errorTemplate.Execute(w, e)
			}
		}()
		fn(w, r)

	}
}

func View(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "image")
	http.ServeFile(w, r, "image-"+r.FormValue("id"))
}

const OAUTH_CLIENT_ID = "YOUR_CLIENT_ID"



var config = &oauth2.Config{
	ClientId:     OAUTH_CLIENT_ID,
	ClientSecret: "YOUR_CLIENT_SECRET",
	Scope:        "https://www.googleapis.com/auth/buzz",
	AuthURL:      "https://accounts.google.com/o/oauth2/auth",
	TokenURL:     "https://accounts.google.com/o/oauth2/token",
	RedirectURL:  "http://moustachio/post",
}
func postPhoto(client *http.Client, photoURL string) os.Error {
 // omitted: url is the URL of the Buzz API
 // req is an API request encoded as JSON
 resp, err := client.Post(url, "application/json", req)
 if err != nil {
 return err
 }
 if resp.StatusCode != 200 {
 return error("invalid post " + resp.Status)
 }
 return nil
}
func share(w http.ResponseWriter, r *http.Request) {
	url := config.AuthCodeURL(r.URL.RawQuery)
	http.Redirect(w, r, url, 302)
}
func post(w http.ResponseWriter, r *http.Request) {
 t := &oauth.Transport{Config: config}
 code := r.FormValue("code")
 _, err := t.Exchange(code)
 check(err)
 image := r.FormValue("state")
 err = postPhoto(t.Client(), "http://moustachio/img?"+image)
 check(err)
 postTemplate.Execute(w, url)
}


func init() {
 http.HandleFunc("/", errorHandler(upload))
 http.HandleFunc("/edit", errorHandler(edit))
 http.HandleFunc("/img", errorHandler(img))
 http.HandleFunc("/share", errorHandler(share))
 http.HandleFunc("/post", errorHandler(post))
}

type Image struct {
 Data []byte
}
func keyOf(data []byte) string {
 sha := sha1.New()
 sha.Write(data)
 return fmt.Sprintf("%x", string(sha.Sum())[0:16])
}



func main() {
	http.HandleFunc("/", errorHandler(upload))
	http.HandleFunc("/View", errorHandler(View))
	fmt.Println("Server is running on :8080")
	http.ListenAndServe(":8080", nil)
}
