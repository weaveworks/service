package registry

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/image"
)

type mockClientAdapter struct {
	imgs []image.Info
	err  error
}

type mockRemote struct {
	img  image.Info
	tags []string
	err  error
}

type ManifestFunc func(id image.Ref) (image.Info, error)
type TagsFunc func(id image.Name) ([]string, error)
type mockDockerClient struct {
	manifest ManifestFunc
	tags     TagsFunc
}

func NewMockClient(manifest ManifestFunc, tags TagsFunc) Client {
	return &mockDockerClient{
		manifest: manifest,
		tags:     tags,
	}
}

func (m *mockDockerClient) Manifest(id image.Ref) (image.Info, error) {
	return m.manifest(id)
}

func (m *mockDockerClient) Tags(id image.Name) ([]string, error) {
	return m.tags(id)
}

func (*mockDockerClient) Cancel() {
	return
}

type mockRemoteFactory struct {
	c   Client
	err error
}

func NewMockClientFactory(c Client, err error) ClientFactory {
	return &mockRemoteFactory{
		c:   c,
		err: err,
	}
}

func (m *mockRemoteFactory) ClientFor(repository string, creds Credentials) (Client, error) {
	return m.c, m.err
}

type mockRegistry struct {
	imgs []image.Info
	err  error
}

func NewMockRegistry(images []image.Info, err error) Registry {
	return &mockRegistry{
		imgs: images,
		err:  err,
	}
}

func (m *mockRegistry) GetRepository(id image.Name) ([]image.Info, error) {
	var imgs []image.Info
	for _, i := range m.imgs {
		// include only if it's the same repository in the same place
		if i.ID.Image == id.Image {
			imgs = append(imgs, i)
		}
	}
	return imgs, m.err
}

func (m *mockRegistry) GetImage(id image.Ref) (image.Info, error) {
	if len(m.imgs) > 0 {
		for _, i := range m.imgs {
			if i.ID.String() == id.String() {
				return i, nil
			}
		}
	}
	return image.Info{}, errors.New("not found")
}
