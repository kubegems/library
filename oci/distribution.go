package oci

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/containers/image/v5/docker/reference"
	specsv1 "github.com/opencontainers/distribution-spec/specs-go/v1"
)

type DistributionOptions struct {
	Username string
	Password string
	Insecure bool // skip tls verify
}

type DistributionOption func(*DistributionOptions)

func WithAuth(username, password string) DistributionOption {
	return func(o *DistributionOptions) {
		o.Username = username
		o.Password = password
	}
}

func WithInsecure() DistributionOption {
	return func(o *DistributionOptions) {
		o.Insecure = true
	}
}

// end-8a	GET	/v2/<name>/tags/list
func ListTags(ctx context.Context, image string, options ...DistributionOption) (*specsv1.TagList, error) {
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, err
	}
	fullpath := reference.Path(named)
	server := "https://" + reference.Domain(named)

	tags := &specsv1.TagList{}
	err = request(ctx, server, http.MethodGet, "/v2/"+fullpath+"/tags/list", nil, tags, options...)
	return tags, err
}

// end-8a

// 参考OCI规范此段实现 https://github.com/opencontainers/distribution-spec/blob/main/spec.md#determining-support
// 目前大部分(所有)镜像仓库均实现了OCI Distribution 规范，可以使用 /v2 接口进行推断，
// 如果认证成功则返回200则认为实现了OCI且认证成功
// end-1	GET	/v2/	200	404/401
func Ping(ctx context.Context, server string, options ...DistributionOption) error {
	err := request(ctx, server, http.MethodGet, "/v2", nil, nil, options...)
	if err != nil {
		return err
	}
	return nil
}

func request(ctx context.Context, server, method, path string,
	postbody interface{}, into interface{}, options ...DistributionOption,
) error {
	opts := &DistributionOptions{}
	for _, o := range options {
		o(opts)
	}

	var body io.Reader
	switch typed := postbody.(type) {
	// convert to bytes
	case []byte:
		body = bytes.NewBuffer(typed)
	// thise type can processed by 'http.NewRequestWithContext(...)'
	case io.Reader:
		body = typed
	case nil:
		// do nothing
	// send json format
	default:
		bts, err := json.Marshal(postbody)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(bts)
	}
	req, err := http.NewRequestWithContext(ctx, method, server+path, body)
	if err != nil {
		return err
	}
	if opts.Password != "" {
		req.SetBasicAuth(opts.Username, opts.Password)
	}
	httpcli := http.DefaultClient
	if opts.Insecure {
		httpcli = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	}
	resp, err := httpcli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errresp := &specsv1.ErrorResponse{}
		bodycontent, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if json.Unmarshal(bodycontent, errresp) != nil {
			// not a json response, return bodycontent as error message
			errresp.Errors = append(errresp.Errors, specsv1.ErrorInfo{
				Code:    resp.Status,
				Message: string(bodycontent),
			})
		}
		return errorResponseError(errresp)
	}
	if into != nil {
		return json.NewDecoder(resp.Body).Decode(into)
	}
	return nil
}

func errorResponseError(err *specsv1.ErrorResponse) error {
	if err == nil {
		return nil
	}

	msg := err.Error() + ":"
	for _, e := range err.Detail() {
		msg += e.Message + ";"
	}
	return errors.New(msg)
}
