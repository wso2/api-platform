/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package soap

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// Namespaces of the documents and elements the rewrite recognizes.
const (
	nsWSDL11       = "http://schemas.xmlsoap.org/wsdl/"
	nsWSDL11SOAP   = "http://schemas.xmlsoap.org/wsdl/soap/"
	nsWSDL11SOAP12 = "http://schemas.xmlsoap.org/wsdl/soap12/"
	nsWSDL11HTTP   = "http://schemas.xmlsoap.org/wsdl/http/"
	nsWSDL20       = "http://www.w3.org/ns/wsdl"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"

	// defaultMaxResponseBytes is the size ceiling when the controller sets none — the
	// router's default per-connection buffer limit, above which the router refuses the
	// response before this policy sees it.
	defaultMaxResponseBytes = 1 << 20
	// maxDepth bounds element nesting. A real service description is a handful of levels
	// deep; this is far above that and far below what would make the parse expensive.
	maxDepth = 256
	// parseTimeout bounds the rewrite of one document, tighter than any request timeout.
	parseTimeout = 2 * time.Second
)

// errRewriteFailed marks a service description that cannot be rewritten safely. The caller
// fails the response rather than forwarding the backend's URLs.
var errRewriteFailed = errors.New("service description cannot be rewritten")

// endpointAttrs maps each endpoint address element (by namespace and local name) to the
// attribute that holds the address. These are the only values rewritten: they are what a
// client generated from the description (SoapUI, wsimport, svcutil) sends its requests to.
// Imports and includes are left as the backend wrote them, as WSO2 API Manager does.
var endpointAttrs = map[xml.Name]string{
	{Space: nsWSDL11SOAP, Local: "address"}:   "location",
	{Space: nsWSDL11SOAP12, Local: "address"}: "location",
	{Space: nsWSDL11HTTP, Local: "address"}:   "location",
	{Space: nsWSDL20, Local: "endpoint"}:      "address",
}

// wsdlRoots are the document elements of a service description. A document with any other
// root is not one this policy touches.
var wsdlRoots = map[xml.Name]bool{
	{Space: nsWSDL11, Local: "definitions"}: true,
	{Space: nsWSDL20, Local: "description"}: true,
}

// wsdlRewriteConfig is one route's rewrite mapping, written by the controller.
type wsdlRewriteConfig struct {
	// gatewayPath is the API's resolved context on the gateway, without a trailing slash
	// ("" for a root context).
	gatewayPath string
	// upstreamBasePath is the upstream URL's unescaped path without a trailing slash (""
	// for none).
	upstreamBasePath string
	// vhost is the virtual host the API is published under; "" or a wildcard means the
	// request's own authority is advertised.
	vhost string
	// maxResponseBytes is the largest service description rewritten.
	maxResponseBytes int
}

// parseWSDLRewriteConfig reads the wsdlRewrite parameter block. It returns (nil, nil) when
// the block is absent. A block that is present but unusable is an error: the controller
// wrote it so the backend's URLs would not be published, and forwarding them anyway is
// exactly what it exists to prevent.
func parseWSDLRewriteConfig(params map[string]any) (*wsdlRewriteConfig, error) {
	raw, present := params[ParamWSDLRewrite]
	if !present || raw == nil {
		return nil, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", ParamWSDLRewrite)
	}

	gatewayPath, _ := block[ParamGatewayPath].(string)
	if !strings.HasPrefix(gatewayPath, "/") {
		return nil, fmt.Errorf("%s.%s must be an absolute path", ParamWSDLRewrite, ParamGatewayPath)
	}
	rawUpstream, _ := block[ParamUpstreamURL].(string)
	upstream, err := url.Parse(rawUpstream)
	if err != nil || (upstream.Scheme != schemeHTTP && upstream.Scheme != schemeHTTPS) || upstream.Host == "" {
		return nil, fmt.Errorf("%s.%s must be an absolute http or https URL", ParamWSDLRewrite, ParamUpstreamURL)
	}
	vhost, _ := block[ParamVhost].(string)

	maxBytes := defaultMaxResponseBytes
	if v, present := block[ParamMaxResponseBytes]; present {
		n, ok := intParam(v)
		if !ok || n < 1 {
			return nil, fmt.Errorf("%s.%s must be a positive integer", ParamWSDLRewrite, ParamMaxResponseBytes)
		}
		maxBytes = n
	}

	return &wsdlRewriteConfig{
		gatewayPath:      strings.TrimSuffix(gatewayPath, "/"),
		upstreamBasePath: strings.TrimSuffix(upstream.Path, "/"),
		vhost:            strings.TrimSpace(vhost),
		maxResponseBytes: maxBytes,
	}, nil
}

// rewrite points every endpoint address in a service description at the gateway.
//
// It returns (nil, nil) when the document is left exactly as it is: not a service
// description, or one with no endpoint address. An error means the document is, or cannot
// be ruled out as, a service description that could not be rewritten safely, and the
// caller must fail rather than forward the backend's addresses.
//
// The document element is read first, so the limits below apply only to a service
// description; any other document a GET returns is forwarded untouched. For a service
// description the parse is bounded per the XXE rules: a size ceiling, no DOCTYPE, a depth
// limit and a deadline. Only the address values are replaced, in place in the original
// bytes, so namespace prefixes, formatting and everything else the backend wrote survive.
func (c *wsdlRewriteConfig) rewrite(ctx context.Context, body []byte, scheme, authority string) ([]byte, error) {
	if looksLikeUTF16XML(body) {
		// Its markup cannot be read byte-wise, so it cannot be told apart from a service
		// description; forwarding it could publish the backend's addresses.
		return nil, fmt.Errorf("%w: UTF-16 documents are not supported", errRewriteFailed)
	}
	if !looksLikeXML(body) {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, parseTimeout)
	defer cancel()

	// Nothing past the ceiling is read: a larger service description is refused at its
	// document element, and a larger document of another kind is classified from this much.
	doc := body
	if len(doc) > c.maxResponseBytes {
		doc = doc[:c.maxResponseBytes]
	}
	// encoding/xml reads only UTF-8. A single-byte encoding is decoded up front, so the
	// offsets the rewrite works with are offsets into the very bytes it rewrites; the
	// result is encoded back before it is returned.
	label, cm, supported := declaredEncoding(doc)
	if !supported {
		// The document element cannot be read, so this may be a service description the
		// gateway cannot rewrite.
		return nil, fmt.Errorf("%w: unsupported encoding %q", errRewriteFailed, label)
	}
	if cm != nil {
		decoded, err := cm.NewDecoder().Bytes(doc)
		if err != nil {
			return nil, fmt.Errorf("%w: cannot decode %s: %v", errRewriteFailed, label, err)
		}
		doc = decoded
	}

	d := xml.NewDecoder(bytes.NewReader(doc))
	d.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		// Reached only for the label declaredEncoding accepted; doc is UTF-8 by now.
		if strings.EqualFold(strings.TrimSpace(charset), label) {
			return input, nil
		}
		return nil, fmt.Errorf("unexpected charset %q", charset)
	}

	var out bytes.Buffer
	last := 0
	depth := 0
	rootSeen := false
	doctypeSeen := false
	rewritten := false
	gatewayBase := ""

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: %v", errRewriteFailed, err)
		}
		start := int(d.InputOffset())
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if rootSeen {
				return nil, fmt.Errorf("%w: malformed document: %v", errRewriteFailed, err)
			}
			if len(body) > c.maxResponseBytes {
				return nil, fmt.Errorf("%w: document element not readable: %v", errRewriteFailed, err)
			}
			// The document element could not be decoded — an entity from a DOCTYPE in one
			// of its attributes, say, which an entity-aware client still reads, endpoint
			// addresses included. Its name decides: a service description is refused, an
			// HTML page passes. With a DOCTYPE and no readable name, it is refused.
			switch name, ok := rawElementLocalName(doc[start:]); {
			case ok && (name == "definitions" || name == "description"):
				return nil, fmt.Errorf("%w: document element not readable: %v", errRewriteFailed, err)
			case !ok && doctypeSeen:
				return nil, fmt.Errorf("%w: document element not readable: %v", errRewriteFailed, err)
			}
			// Not well-formed before or at its first element — an HTML page, say: not a
			// service description.
			return nil, nil
		}

		switch t := tok.(type) {
		case xml.Directive:
			if !rootSeen && bytes.HasPrefix(bytes.TrimSpace(t), []byte("DOCTYPE")) {
				doctypeSeen = true
			}
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return nil, fmt.Errorf("%w: nesting deeper than %d", errRewriteFailed, maxDepth)
			}
			if !rootSeen {
				rootSeen = true
				if !wsdlRoots[t.Name] {
					return nil, nil
				}
				if len(body) > c.maxResponseBytes {
					return nil, fmt.Errorf("%w: document of %d bytes exceeds the %d-byte ceiling", errRewriteFailed, len(body), c.maxResponseBytes)
				}
				if doctypeSeen {
					return nil, fmt.Errorf("%w: document carries a DOCTYPE declaration", errRewriteFailed)
				}
			}
			attr, ok := endpointAttrs[t.Name]
			if !ok {
				continue
			}
			value, ok := unqualifiedAttr(t, attr)
			if !ok {
				continue
			}
			if gatewayBase == "" {
				if gatewayBase, err = c.gatewayOrigin(scheme, authority); err != nil {
					return nil, err
				}
			}
			end := int(d.InputOffset())
			span := attrValueSpan(doc[start:end], attr)
			if span == nil {
				return nil, fmt.Errorf("%w: cannot locate %s on <%s>", errRewriteFailed, attr, t.Name.Local)
			}
			out.Write(doc[last : start+span[0]])
			xmlEscapeAttr(&out, c.endpointURL(value, gatewayBase))
			last = start + span[1]
			rewritten = true
		case xml.EndElement:
			depth--
		}
	}

	if !rewritten {
		return nil, nil
	}
	out.Write(doc[last:])
	if cm == nil {
		return out.Bytes(), nil
	}
	encoded, err := cm.NewEncoder().Bytes(out.Bytes())
	if err != nil {
		// A byte the code page leaves undefined decoded to U+FFFD, which it cannot write.
		return nil, fmt.Errorf("%w: cannot re-encode as %s: %v", errRewriteFailed, label, err)
	}
	return encoded, nil
}

// endpointURL is the gateway address that replaces a backend endpoint address. Every
// address is replaced, whatever host it names: a backend often advertises a name other
// than the one the gateway reaches it by, and an address left as it was would send
// clients around the gateway.
//
// An address below the upstream's base path keeps the part after it (and its query), since
// the API's routes carry sub-paths through to the backend. Anything else — another path, a
// relative value, a placeholder such as "REPLACE_WITH_ACTUAL_URL" — becomes the API's
// context, as WSO2 API Manager does.
func (c *wsdlRewriteConfig) endpointURL(value, gatewayBase string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err == nil && (u.Scheme == schemeHTTP || u.Scheme == schemeHTTPS) && u.Host != "" {
		// Compared unescaped on both sides, so an upstream path such as "/my svc" matches
		// the "/my%20svc" a description advertises.
		if suffix, under := pathUnder(u.Path, c.upstreamBasePath); under {
			result := gatewayBase + escapedPath(c.gatewayPath+suffix)
			if u.RawQuery != "" {
				result += "?" + u.RawQuery
			}
			return result
		}
	}
	return gatewayBase + escapedPath(c.gatewayPath)
}

// escapedPath is p percent-encoded for a URL, and "/" for the empty path.
func escapedPath(p string) string {
	if p == "" {
		return "/"
	}
	return (&url.URL{Path: p}).EscapedPath()
}

// pathUnder reports whether path is basePath or below it, and returns the part after it.
func pathUnder(path, basePath string) (string, bool) {
	if basePath == "" {
		if path == "/" {
			return "", true
		}
		return path, true
	}
	if path == basePath || path == basePath+"/" {
		return strings.TrimPrefix(path, basePath), true
	}
	if strings.HasPrefix(path, basePath+"/") {
		return path[len(basePath):], true
	}
	return "", false
}

// gatewayOrigin builds the scheme and authority a rewritten URL advertises.
//
// scheme and authority come from the request, so both are validated rather than
// interpolated: the result is published to every client that fetches the description.
func (c *wsdlRewriteConfig) gatewayOrigin(scheme, requestAuthority string) (string, error) {
	if scheme != schemeHTTP && scheme != schemeHTTPS {
		return "", fmt.Errorf("%w: request scheme %q is not http or https", errRewriteFailed, scheme)
	}
	authority := gatewayAuthority(c.vhost, requestAuthority, scheme)
	if !validAuthority(authority) {
		return "", fmt.Errorf("%w: no usable gateway authority for the request", errRewriteFailed)
	}
	return scheme + "://" + authority, nil
}

// gatewayAuthority composes the advertised authority from the API's virtual host and the
// request. The vhost decides the host — the name the operator published the API under,
// rather than whatever internal name a client happened to dial — and the request supplies
// the port, which the vhost does not name. A standard port is left off. A wildcard vhost
// names no dialable host, so the request's own authority is used whole. (The same rule the
// A2A system policy applies to a rewritten Agent Card.)
func gatewayAuthority(vhost, requestAuthority, scheme string) string {
	if vhost == "" || strings.Contains(vhost, "*") {
		return requestAuthority
	}
	_, port, err := net.SplitHostPort(requestAuthority)
	if err != nil || port == "" || isStandardPort(scheme, port) {
		return vhost
	}
	return net.JoinHostPort(vhost, port)
}

func isStandardPort(scheme, port string) bool {
	return (scheme == schemeHTTP && port == "80") || (scheme == schemeHTTPS && port == "443")
}

// validAuthority rejects an authority that cannot be published as one: empty, carrying a
// path, userinfo or wildcard, or carrying whitespace or control characters.
func validAuthority(authority string) bool {
	if authority == "" || len(authority) > 255 {
		return false
	}
	if strings.ContainsAny(authority, "/?#@*\\ \t\r\n") {
		return false
	}
	for _, r := range authority {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// rawElementLocalName reads the local name of the start tag that b begins with (after
// whitespace), without decoding it. ok is false when b does not begin with a start tag.
func rawElementLocalName(b []byte) (string, bool) {
	b = bytes.TrimLeft(b, " \t\r\n")
	if len(b) < 2 || b[0] != '<' || b[1] == '!' || b[1] == '?' || b[1] == '/' {
		return "", false
	}
	end := 1
	for end < len(b) && !isXMLSpace(b[end]) && b[end] != '/' && b[end] != '>' {
		end++
	}
	name := string(b[1:end])
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}
	return name, name != ""
}

// unqualifiedAttr returns the value of an attribute with no namespace prefix. The location
// attributes are all unqualified; a prefixed one of the same local name (xsi:schemaLocation,
// say) is a different attribute.
func unqualifiedAttr(t xml.StartElement, name string) (string, bool) {
	for _, a := range t.Attr {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

// attrValueSpan locates an unqualified attribute's value inside a raw start tag, returning
// the [start, end) offsets of the text between its quotes, or nil. The tag is walked
// attribute by attribute, so text inside another attribute's quoted value is never taken
// for the attribute sought, and "location" never matches a prefixed "wsdl:location". The
// decoder has already accepted the tag, so it is well-formed.
func attrValueSpan(rawTag []byte, name string) []int {
	i := 1 // past '<'
	for i < len(rawTag) && !isXMLSpace(rawTag[i]) && rawTag[i] != '/' && rawTag[i] != '>' {
		i++
	}
	for {
		for i < len(rawTag) && isXMLSpace(rawTag[i]) {
			i++
		}
		if i >= len(rawTag) || rawTag[i] == '/' || rawTag[i] == '>' {
			return nil
		}
		nameStart := i
		for i < len(rawTag) && !isXMLSpace(rawTag[i]) && rawTag[i] != '=' {
			i++
		}
		attrName := string(rawTag[nameStart:i])
		for i < len(rawTag) && isXMLSpace(rawTag[i]) {
			i++
		}
		if i >= len(rawTag) || rawTag[i] != '=' {
			return nil
		}
		i++
		for i < len(rawTag) && isXMLSpace(rawTag[i]) {
			i++
		}
		if i >= len(rawTag) || (rawTag[i] != '"' && rawTag[i] != '\'') {
			return nil
		}
		quote := rawTag[i]
		i++
		valueStart := i
		for i < len(rawTag) && rawTag[i] != quote {
			i++
		}
		if i >= len(rawTag) {
			return nil
		}
		if attrName == name {
			return []int{valueStart, i}
		}
		i++
	}
}

func isXMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// xmlEscapeAttr writes s escaped for a double- or single-quoted attribute value.
func xmlEscapeAttr(b *bytes.Buffer, s string) {
	_ = xml.EscapeText(b, []byte(s))
}

// looksLikeXML reports whether a body starts, after an optional BOM and whitespace, with
// markup. A JSON or HTML-less plain-text body is not a document this policy reads.
func looksLikeXML(body []byte) bool {
	b := bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	b = bytes.TrimLeft(b, " \t\r\n")
	return len(b) > 0 && b[0] == '<'
}

// looksLikeUTF16XML reports whether a body is markup encoded in UTF-16: its first
// character, after an optional byte order mark, is '<' in either byte order.
func looksLikeUTF16XML(body []byte) bool {
	b := body
	switch {
	case bytes.HasPrefix(b, []byte{0xfe, 0xff}), bytes.HasPrefix(b, []byte{0xff, 0xfe}):
		b = b[2:]
	}
	return len(b) >= 2 && ((b[0] == '<' && b[1] == 0) || (b[0] == 0 && b[1] == '<'))
}

// declaredEncoding reads the encoding named by the XML declaration. It returns the label
// as written and, for a single-byte code page, the charmap that decodes it; the charmap is
// nil for UTF-8, US-ASCII (a subset of it) or no declaration. supported is false for any
// other encoding: its markup cannot be read reliably byte-wise.
func declaredEncoding(doc []byte) (label string, cm *charmap.Charmap, supported bool) {
	b := bytes.TrimPrefix(doc, []byte("\xef\xbb\xbf"))
	if !bytes.HasPrefix(b, []byte("<?xml")) {
		return "", nil, true
	}
	end := bytes.Index(b, []byte("?>"))
	if end < 0 {
		// Malformed; the decoder reports it.
		return "", nil, true
	}
	label, ok := pseudoAttr(string(b[len("<?xml"):end]), "encoding")
	if !ok {
		return "", nil, true
	}
	cs := strings.ToLower(strings.TrimSpace(label))
	switch cs {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return label, nil, true
	case "latin1", "latin-1":
		return label, charmap.ISO8859_1, true
	}
	for _, prefix := range []string{"iso-8859-", "iso8859-", "iso_8859-"} {
		if n, ok := strings.CutPrefix(cs, prefix); ok {
			cm, supported := iso8859[n]
			return label, cm, supported
		}
	}
	for _, prefix := range []string{"windows-", "cp"} {
		if n, ok := strings.CutPrefix(cs, prefix); ok {
			cm, supported := windowsCodePages[n]
			return label, cm, supported
		}
	}
	return label, nil, false
}

// pseudoAttr returns a pseudo-attribute's value from the inside of an XML declaration.
func pseudoAttr(decl, name string) (string, bool) {
	i := strings.Index(decl, name)
	for i >= 0 {
		rest := strings.TrimLeft(decl[i+len(name):], " \t\r\n")
		if strings.HasPrefix(rest, "=") {
			rest = strings.TrimLeft(rest[1:], " \t\r\n")
			if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
				if j := strings.IndexByte(rest[1:], rest[0]); j >= 0 {
					return rest[1 : 1+j], true
				}
			}
			return "", false
		}
		next := strings.Index(decl[i+1:], name)
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return "", false
}

// The single-byte code pages a service description may declare, by the number after the
// family prefix.
var (
	iso8859 = map[string]*charmap.Charmap{
		"1": charmap.ISO8859_1, "2": charmap.ISO8859_2, "3": charmap.ISO8859_3, "4": charmap.ISO8859_4,
		"5": charmap.ISO8859_5, "6": charmap.ISO8859_6, "7": charmap.ISO8859_7, "8": charmap.ISO8859_8,
		"9": charmap.ISO8859_9, "10": charmap.ISO8859_10, "13": charmap.ISO8859_13, "14": charmap.ISO8859_14,
		"15": charmap.ISO8859_15, "16": charmap.ISO8859_16,
	}
	windowsCodePages = map[string]*charmap.Charmap{
		"1250": charmap.Windows1250, "1251": charmap.Windows1251, "1252": charmap.Windows1252,
		"1253": charmap.Windows1253, "1254": charmap.Windows1254, "1255": charmap.Windows1255,
		"1256": charmap.Windows1256, "1257": charmap.Windows1257, "1258": charmap.Windows1258,
	}
)

// intParam reads an integer parameter, which arrives as a float64 when it came from JSON.
func intParam(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}
