package impersonator

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/textproto"
)

type insertMetadata struct {
	LabelIDs []string `json:"labelIds"`
}

func createMultipartBody(content io.Reader, labelIDs []string) (*io.PipeReader, string) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)

	contentType := "multipart/related; boundary=" + multipartWriter.Boundary()

	go func() {
		metadatadaHeader := make(textproto.MIMEHeader)
		metadatadaHeader.Set("Content-Type", "application/json; charset=UTF-8")

		metadataPart, err := multipartWriter.CreatePart(metadatadaHeader)
		if err != nil {
			writer.CloseWithError(err)
			return
		}

		metadata := insertMetadata{
			LabelIDs: labelIDs,
		}

		if err := json.NewEncoder(metadataPart).Encode(metadata); err != nil {
			writer.CloseWithError(err)
			return
		}

		emailHeader := make(textproto.MIMEHeader)
		emailHeader.Set("Content-Type", "message/rfc822")

		emailPart, err := multipartWriter.CreatePart(emailHeader)
		if err != nil {
			writer.CloseWithError(err)
			return
		}

		if _, err := io.Copy(emailPart, content); err != nil {
			writer.CloseWithError(err)
			return
		}

		if err := multipartWriter.Close(); err != nil {
			writer.CloseWithError(err)
			return
		}

		writer.Close()
	}()

	return reader, contentType
}
