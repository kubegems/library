package oci

import (
	"strings"

	"github.com/containers/image/v5/docker/reference"
)

// barbor.foo.com/project/artifact:tag -> barbor.foo.com,project,artifact,tag
// barbor.foo.com/project/foo/artifact:tag -> barbor.foo.com,project/foo,artifact,tag
// barbor.foo.com/artifact:tag -> barbor.foo.com,library,artifact,tag
// project/artifact:tag -> docker.io,project,artifact,tag
func ParseImage(image string) (domain, path, name, tag string, err error) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return
	}
	domain, fullpath := reference.Domain(named), reference.Path(named)
	if i := strings.LastIndex(fullpath, "/"); i != -1 {
		path, name = fullpath[:i], fullpath[i+1:]
	} else {
		path, name = "library", fullpath
	}
	if tagged, ok := named.(reference.Tagged); ok {
		tag = tagged.Tag()
	}
	if tagged, ok := named.(reference.Digested); ok {
		tag = tagged.Digest().String()
	}
	if tag == "" {
		tag = "latest"
	}
	return domain, path, name, tag, nil
}
