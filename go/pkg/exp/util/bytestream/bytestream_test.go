package bytestream

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"testing"
	"testing/quick"

	"zenhack.net/go/tempest/capnp/util"
)

type writeNoopCloser struct {
	io.Writer
}

func (w writeNoopCloser) Close() error {
	return nil
}

// compute the md5 sum of p both directly and by writing it through
// the bytestream interface, returns true if the results are the same.
// We take a *testing.T for the logging facilities, but it is never
// flagged as failing.
func checkMd5(t *testing.T, data []byte) bool {
	if data == nil {
		// We don't want to deal with this
		return true
	}
	hash := md5.New()
	hash.Write(data)
	directDigest := fmt.Sprintf("%x", hash.Sum([]byte{}))

	hash.Reset()
	t.Logf("Direct digest: %q\n", directDigest)

	ctx := context.Background()

	bsClient := FromWriteCloser(writeNoopCloser{hash}, nil)
	writeRes, release := bsClient.Write(ctx, func(p util.ByteStream_write_Params) error {
		p.SetData(data)
		return nil
	})
	defer release()
	_, err := writeRes.Struct()
	if err != nil {
		t.Logf("Error: %v", err)
		return false
	}
	doneRes, release := bsClient.Done(ctx, func(p util.ByteStream_done_Params) error {
		return nil
	})
	defer release()
	_, err = doneRes.Struct()
	if err != nil {
		t.Logf("Error: %v", err)
		return false
	}

	rpcDigest := fmt.Sprintf("%x", hash.Sum([]byte{}))
	t.Logf("RPC digest: %q\n", rpcDigest)

	return rpcDigest == directDigest
}

func TestMd5(t *testing.T) {
	err := quick.Check(func(data []byte) bool {
		return checkMd5(t, data)
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}
