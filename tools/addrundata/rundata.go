package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
)

// markdownRE matches the heading line: `# XX-1.1: Foo Functional Test`
var markdownRE = regexp.MustCompile(`#(.*?):(.*)`)

// parseMarkdown reads metadata from README.md.
func parseMarkdown(r io.Reader) (*mpb.Metadata, error) {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("missing markdown heading")
	}
	line := sc.Text()
	m := markdownRE.FindStringSubmatch(line)
	if len(m) < 3 {
		return nil, fmt.Errorf("cannot parse markdown: %s", line)
	}
	return &mpb.Metadata{
		PlanId:      strings.TrimSpace(m[1]),
		Description: strings.TrimSpace(m[2]),
	}, nil
}

// parseCode reads metadata from a source code.
func parseCode(r io.Reader) (*mpb.Metadata, error) {
	var md *mpb.Metadata
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if line := sc.Text(); line != "func init() {" {
			continue
		}
		var err error
		md, err = parseInit(sc)
		if err != nil {
			return nil, err
		}
		break
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if md == nil {
		return nil, errors.New("missing func init()")
	}
	return md, nil
}

// parseProto reads metadata from a textproto.
func parseProto(r io.Reader) (*mpb.Metadata, error) {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	md := new(mpb.Metadata)
	return md, prototext.Unmarshal(bytes, md)
}

// rundataRE matches a line like this: `  rundata.TestUUID = "..."`
var rundataRE = regexp.MustCompile(`\s+rundata\.(\w+) = (".*")`)

// parseInit parses the rundata from the body of func init().
func parseInit(sc *bufio.Scanner) (*mpb.Metadata, error) {
	md := new(mpb.Metadata)
	for sc.Scan() {
		line := sc.Text()
		if line == "}" {
			return md, nil
		}
		m := rundataRE.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		k := m[1]
		v, err := strconv.Unquote(m[2])
		if err != nil {
			return nil, fmt.Errorf("cannot parse rundata line: %s: %w", line, err)
		}
		switch k {
		case "TestPlanID":
			md.PlanId = v
		case "TestDescription":
			md.Description = v
		case "TestUUID":
			md.Uuid = v
		}
	}
	return nil, errors.New("func init() was not terminated")
}

var marshaller = prototext.MarshalOptions{Multiline: true}

// writeProto generates a complete metadata.textproto to the writer.
func writeProto(w io.Writer, md *mpb.Metadata) error {
	const header = `# proto-file: github.com/openconfig/featureprofiles/proto/metadata.proto
# proto-message: Metadata

`
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	bytes, err := marshaller.Marshal(md)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}
