package enmime

import (
	"bufio"
	"bytes"
	"fmt"
	"mime"
	"net/textproto"
	"strings"
)

const (
	// Standard MIME content dispositions
	cdAttachment = "attachment"
	cdInline     = "inline"

	// Standard MIME content types
	ctAppOctetStream  = "application/octet-stream"
	ctMultipartAltern = "multipart/altern"
	ctMultipartPrefix = "multipart/"
	ctTextPlain       = "text/plain"
	ctTextHTML        = "text/html"

	// Standard MIME header names
	hnContentDisposition = "Content-Disposition"
	hnContentEncoding    = "Content-Transfer-Encoding"
	hnContentType        = "Content-Type"

	// Standard MIME header parameters
	hpBoundary = "boundary"
	hpCharset  = "charset"
	hpFile     = "file"
	hpFilename = "filename"
	hpName     = "name"
)

// AddressHeaders is the set of SMTP headers that contain email addresses, used by
// Envelope.AddressList().  Key characters must be all lowercase.
var AddressHeaders = map[string]bool{
	"bcc":          true,
	"cc":           true,
	"delivered-to": true,
	"from":         true,
	"reply-to":     true,
	"to":           true,
}

func debug(format string, args ...interface{}) {
	if false {
		fmt.Printf(format, args...)
		fmt.Println()
	}
}

// Terminology from RFC 2047:
//  encoded-word: the entire =?charset?encoding?encoded-text?= string
//  charset: the character set portion of the encoded word
//  encoding: the character encoding type used for the encoded-text
//  encoded-text: the text we are decoding

// readHeader reads a block of SMTP or MIME headers and returns a textproto.MIMEHeader.
// Header parse warnings & errors will be added to p.Errors, io errors will be returned directly.
func readHeader(r *bufio.Reader, p *Part) (textproto.MIMEHeader, error) {
	// buf holds the massaged output for textproto.Reader.ReadMIMEHeader()
	buf := &bytes.Buffer{}

	for {
		lineBuf, err := r.ReadSlice('\n')
		if err != nil {
			return nil, err
		}
		if len(lineBuf) == 0 || lineBuf[0] == '\r' || lineBuf[0] == '\n' {
			// End of headers
			break
		}
		buf.Write(lineBuf)
	}
	buf.Write([]byte{'\r', '\n'})

	// Parse the massaged header using textproto package
	tr := textproto.NewReader(bufio.NewReader(buf))
	header, err := tr.ReadMIMEHeader()
	return header, err
}

// decodeHeader decodes a single line (per RFC 2047) using Golang's mime.WordDecoder
func decodeHeader(input string) string {
	if !strings.Contains(input, "=?") {
		// Don't scan if there is nothing to do here
		return input
	}

	dec := new(mime.WordDecoder)
	dec.CharsetReader = newCharsetReader
	header, err := dec.DecodeHeader(input)
	if err != nil {
		return input
	}
	return header
}

// decodeToUTF8Base64Header decodes a MIME header per RFC 2047, reencoding to =?utf-8b?
func decodeToUTF8Base64Header(input string) string {
	if !strings.Contains(input, "=?") {
		// Don't scan if there is nothing to do here
		return input
	}

	debug("input = %q", input)
	tokens := strings.FieldsFunc(input, isWhiteSpaceRune)
	output := make([]string, len(tokens), len(tokens))
	for i, token := range tokens {
		if len(token) > 4 && strings.Contains(token, "=?") {
			// Stash parenthesis, they should not be encoded
			prefix := ""
			suffix := ""
			if token[0] == '(' {
				prefix = "("
				token = token[1:]
			}
			if token[len(token)-1] == ')' {
				suffix = ")"
				token = token[:len(token)-1]
			}
			// Base64 encode token
			output[i] = prefix + mime.BEncoding.Encode("UTF-8", decodeHeader(token)) + suffix
		} else {
			output[i] = token
		}
		debug("%v %q %q", i, token, output[i])
	}

	// Return space separated tokens
	return strings.Join(output, " ")
}

// Detects a RFC-822 linear-white-space, passed to strings.FieldsFunc
func isWhiteSpaceRune(r rune) bool {
	switch r {
	case ' ':
		return true
	case '\t':
		return true
	case '\r':
		return true
	case '\n':
		return true
	default:
		return false
	}
}
