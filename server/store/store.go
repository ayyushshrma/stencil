package store

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"strings"

	"github.com/odpf/stencil/server/config"
	"gocloud.dev/blob"

	// Required by blob module
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/memblob"
)

func directoryFilter(obj *blob.ListObject) bool {
	return obj.IsDir
}

func fileFilter(obj *blob.ListObject) bool {
	return !obj.IsDir
}

func filterMap(prefix string, filter func(*blob.ListObject) bool) func(*blob.ListObject) (bool, string) {
	return func(obj *blob.ListObject) (bool, string) {
		if ok := filter(obj); !ok {
			return false, ""
		}
		key := path.Join(strings.Replace(obj.Key, fmt.Sprintf("%s", prefix), "", 1))
		return key != "", key
	}
}

//Store Backend storage
type Store struct {
	Bucket *blob.Bucket
}

func (s *Store) list(prefix string, filterMap func(*blob.ListObject) (bool, string)) ([]string, error) {
	ctx := context.Background()
	options := &blob.ListOptions{Prefix: prefix, Delimiter: "/"}
	listIter := s.Bucket.List(options)
	keys := []string{}
	for {
		obj, err := listIter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if ok, key := filterMap(obj); ok {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

//ListDir returns list of directories matching with prefix
func (s *Store) ListDir(prefix string) ([]string, error) {
	return s.list(prefix, filterMap(prefix, directoryFilter))
}

//ListFiles returns list of files matching with prefix
func (s *Store) ListFiles(prefix string) ([]string, error) {
	return s.list(prefix, filterMap(prefix, fileFilter))
}

//Put Uploads file from r io.Reader with specified name
func (s *Store) Put(ctx context.Context, filename string, r io.Reader) error {
	w, err := s.Bucket.NewWriter(ctx, filename, nil)
	if err != nil {
		return err
	}
	_, err = w.ReadFrom(r)
	if err != nil {
		return err
	}
	err = w.Close()
	return err
}

//Get Download file
func (s *Store) Get(ctx context.Context, filename string) (*blob.Reader, error) {
	reader, err := s.Bucket.NewReader(ctx, filename, nil)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

//Copy copy one file to another file
func (s *Store) Copy(ctx context.Context, fromFile, toFile string) error {
	reader, err := s.Get(ctx, fromFile)
	if err != nil {
		return err
	}
	defer reader.Close()
	return s.Put(ctx, toFile, reader)
}

//Close Closes bucket connection
func (s *Store) Close() {
	s.Bucket.Close()
}

// New creates a new store
func New(c *config.Config) *Store {
	ctx := context.Background()
	url := c.BucketURL
	bucket, err := blob.OpenBucket(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	store := Store{
		Bucket: bucket,
	}
	return &store
}