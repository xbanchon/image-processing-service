package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/h2non/bimg"
	"github.com/xbanchon/image-processing-service/internal/processor"
	"github.com/xbanchon/image-processing-service/internal/store"
)

var ImageTypes = map[string]bimg.ImageType{
	"jpeg": bimg.JPEG,
	"png":  bimg.PNG,
	"webp": bimg.WEBP,
	"tiff": bimg.TIFF,
}

type Metadata struct {
	Width  int
	Height int
	Format string `json:"format"`
	Size   int64  `json:"size"`
}

type Image struct {
	URL      string //URL of image in object storage
	Metadata Metadata
}

type RequestPayload struct {
	Transformations `json:"transformations"`
}

type Transformations struct {
	Resize ResizeParams `json:"resize"`
	Crop   CropParams   `json:"crop"`
	// Watermark WatermarkParams `json:"watermark"`
	Mirror  bool   `json:"mirror"` //Mirror image about Y-axis
	Flip    bool   `json:"flip"`   //Mirror image about X-axis
	Rotate  int    `json:"rotate"`
	Quality int    `json:"quality"` //Compress final image
	Format  string `json:"format"`  //Image format e.g.: JPG, PNG,...
	Filters struct {
		Grayscale    bool    `json:"grayscale"`
		Sepia        bool    `json:"sepia"`
		Gamma        float32 `json:"gamma"`
		GaussianBlur float32 `json:"gaussian_blur"`
	} `json:"filters"`
}

type CropParams struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ResizeParams struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type WatermarkParams struct {
}

// type Filters struct {
// 	GaussianBlurParams `json:"gaussian_blur"`
// 	Grayscale          bool    `json:"grayscale"`
// 	Sepia              bool    `json:"sepia"`
// 	Gamma              float64 `json:"gamma"`
// 	// add more filters
// }

// type GaussianBlurParams struct {
// 	Sigma   float64 `json:"sigma"`
// 	MinAmpl float64 `json:"min_ampl"`
// }

func (app *application) uploadImageHandler(w http.ResponseWriter, r *http.Request) {
	buf, filename, _, err := readImageData(r)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	bucketFilename, signedURL, err := app.bucket.Images.UploadImage(filename, buf) //error when uploading to supabase. Returning empty strings as consequence
	if err != nil {
		log.Println("upload to bucket error")
		app.internalServerError(w, r, err)
		return
	}
	app.logger.Info("image uploaded succesfully")

	if bucketFilename == "" || signedURL == "" {
		log.Println("bad upload to bucket")
		return
	}

	user := getUserFromContext(r)
	log.Printf("User [%v] request", user.Username)

	image := &store.Image{
		URL:      signedURL,
		Filename: bucketFilename,
		UserID:   user.ID,
	}

	ctx := r.Context()

	if err := app.store.Images.Create(ctx, image); err != nil { //duplicate insertion to database. Investigate why
		log.Println("insert to database error")
		app.internalServerError(w, r, err)
		return
	}
	app.logger.Info("image register added")

	if err := app.jsonResponse(w, http.StatusCreated, image); err != nil {
		log.Println("server response error")
		app.internalServerError(w, r, err)
	}
}

func (app *application) getImageHandler(w http.ResponseWriter, r *http.Request) {
	imageID, err := strconv.ParseInt(chi.URLParam(r, "imageID"), 10, 64)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	image, err := app.getImage(r.Context(), imageID)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.notFoundResponse(w, r, err)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	log.Printf("requested image: %+v", image)
	if image == nil || image.UserID == 0 {
		app.internalServerError(w, r, errors.New("unknown cache error"))
		return
	}

	user := getUserFromContext(r)
	if user.ID != image.UserID {
		app.unauthorizedErrorResponse(w, r, err)
		return
	}

	if err := app.jsonResponse(w, http.StatusOK, image); err != nil {
		app.internalServerError(w, r, err)
	}
}

func (app *application) getImagesHandler(w http.ResponseWriter, r *http.Request) {
	pp := store.PaginationParams{
		PageID: 1,
		Limit:  10,
	}
	pp, err := pp.Parse(r)
	// log.Printf("pagination params: %+v", pp)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(pp); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	ctx := r.Context()
	user := getUserFromContext(r)

	images, err := app.store.Images.GetUserImages(ctx, user.ID, pp)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := app.jsonResponse(w, http.StatusOK, images); err != nil {
		app.internalServerError(w, r, err)
	}
}

func (app *application) transformImageHandler(w http.ResponseWriter, r *http.Request) {
	imageID, err := strconv.ParseInt(chi.URLParam(r, "imageID"), 10, 64)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	image, err := app.getImage(r.Context(), imageID)
	if err != nil {
		switch err {
		case store.ErrNotFound:
			app.notFoundResponse(w, r, err)
		default:
			app.internalServerError(w, r, err)
		}
		return
	}

	user := getUserFromContext(r)
	if image.UserID != user.ID {
		app.forbiddenResponse(w, r, err)
		return
	}

	buf, err := app.bucket.Images.StreamImage(image.Filename)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	var payload RequestPayload

	if err := readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	options := processor.Transformer{
		Resize: struct {
			Width  int
			Height int
		}(payload.Resize),
		Crop: struct {
			Width  int
			Height int
		}(payload.Crop),
		Mirror:  payload.Mirror,
		Flip:    payload.Flip,
		Rotate:  payload.Rotate,
		Quality: payload.Quality,
		Format:  payload.Format,
		Filters: struct {
			Grayscale    bool
			Sepia        bool
			Gamma        float32
			GaussianBlur float32
		}(payload.Filters),
	}

	log.Printf("user [%v] request -> image [%d] transformation ops: %+v", user.Username, image.ID, payload)
	ip := processor.NewImageProcessor(buf, options)
	newBuf, err := ip.Transformer.Process()
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := app.bucket.Images.UpdateImage(image.Filename, newBuf); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	//update image info
	image.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := app.store.Images.Update(r.Context(), image); err != nil {
		app.internalServerError(w, r, err)
		return
	}

	if err := app.jsonResponse(w, http.StatusOK, image); err != nil {
		app.internalServerError(w, r, err)
	}
}

// Test endpoints
func (app *application) testMetadataEndpoint(w http.ResponseWriter, r *http.Request) {
	buf, filename, size, err := readImageData(r)
	if err != nil {
		app.internalServerError(w, r, err)
	}

	metadata, err := getImageMetadata(buf)
	if err != nil {
		app.internalServerError(w, r, err)
	}

	type CustomMetadata struct {
		Width   int    `json:"width"`
		Height  int    `json:"height"`
		ImgType string `json:"imgtype"`
	}

	type envelope struct {
		Filename string         `json:"filename"`
		Size     int64          `json:"size"`
		Metadata CustomMetadata `json:"metadata"`
	}

	newMetadata := CustomMetadata{
		Width:   metadata.Size.Width,
		Height:  metadata.Size.Height,
		ImgType: metadata.Type,
	}

	json.NewEncoder(w).Encode(&envelope{
		Filename: filename,
		Size:     size,
		Metadata: newMetadata,
	})
}

func (app *application) testBasicTransformation(w http.ResponseWriter, r *http.Request) {
	buf, filename, _, err := readImageData(r)
	if err != nil {
		app.internalServerError(w, r, err)
		log.Printf("internal error: %v", err.Error())
		return
	}

	newImage, err := bimg.NewImage(buf).Rotate(90)
	if err != nil {
		app.internalServerError(w, r, err)
		log.Printf("internal error: %v", err.Error())
		return
	}

	err = bimg.Write("/opt/projects/image-processing-service/testdata/out/transform_"+filename, newImage)
	if err != nil {
		app.internalServerError(w, r, err)
		log.Printf("internal error: %v", err.Error())
		return
	}

	app.jsonResponse(w, http.StatusCreated, "image transformed successfully!")
}

func (app *application) testImageURL(w http.ResponseWriter, r *http.Request) {
	duration := 6 * 3600 // 6 hours
	url, err := app.bucket.Images.GetNewSignedImageURL("uploaded_crossing_line2.jpg", duration)
	if err != nil {
		app.internalServerError(w, r, err)
		return
	}
	if url == "" {
		app.jsonResponse(w, http.StatusBadRequest, "failed to get url")
		return
	}
	app.jsonResponse(w, http.StatusFound, url)

}

func (app *application) testTransformationReq(w http.ResponseWriter, r *http.Request) {
	var payload RequestPayload
	if err := readJSON(w, r, &payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if err := Validate.Struct(payload); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	trReq := &processor.Transformer{
		Resize: struct {
			Width  int
			Height int
		}(payload.Resize),
		Crop: struct {
			Width  int
			Height int
		}(payload.Crop),
		Mirror:  payload.Mirror,
		Flip:    payload.Flip,
		Rotate:  payload.Rotate,
		Quality: payload.Quality,
		Format:  payload.Format,
		Filters: struct {
			Grayscale    bool
			Sepia        bool
			Gamma        float32
			GaussianBlur float32
		}(payload.Filters),
	}

	if err := app.jsonResponse(w, http.StatusOK, trReq); err != nil {
		app.internalServerError(w, r, err)
		return
	}
}

// Utils
func getImageMetadata(buffer []byte) (bimg.ImageMetadata, error) {
	metadata, err := bimg.Metadata(buffer)

	if err != nil {
		return bimg.ImageMetadata{}, err
	}

	return metadata, nil
}

func readImageData(r *http.Request) ([]byte, string, int64, error) {
	r.ParseMultipartForm(10 >> 20) // 10MB
	r.ParseForm()
	image, header, err := r.FormFile("image")

	if err != nil {
		return nil, "", 0, err
	}

	defer image.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, image); err != nil {
		return nil, "", 0, err
	}

	return buf.Bytes(), header.Filename, header.Size, nil
}
