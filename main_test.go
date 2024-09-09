package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	_ "embed"

	sgbucket "github.com/couchbase/sg-bucket"
	"github.com/couchbaselabs/walrus"
	extism "github.com/extism/go-sdk"
	"github.com/stretchr/testify/require"
)

//go:embed "wasm/01_emit/dist/plugin.wasm"
var simple []byte

//go:embed "wasm/jq/target/wasm32-wasi/release/plugin.wasm"
var jq []byte

func TestSimple(t *testing.T) {
	key := "some-key"
	value := "some-value"

	type MyDoc struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{extism.WasmData{Data: simple}},
	}
	fn, err := json.Marshal(manifest)
	require.NoError(t, err)

	bucket := walrus.NewBucket("bucketname")
	bucket.Add("key", 0, &MyDoc{Key: key, Value: value})
	bucket.PutDDoc(context.Background(), "key", &sgbucket.DesignDoc{
		Views: sgbucket.ViewMap{
			"my-view": sgbucket.ViewDef{
				Map: string(fn),
			},
		},
	})
	result, err := bucket.View(context.Background(), "key", "my-view", nil)
	if err != nil {
		panic(err)
	}

	for _, r := range result.Rows {
		require.Equal(t, key, r.Key)
		require.Equal(t, strings.ToUpper(value), r.Value)
		fmt.Printf("key: %s, value: %s\n", r.Key, r.Value)
	}

}

func TestJq(t *testing.T) {
	type User struct {
		UserId string `json:"user-id"`
		Email  string `json:"email"`
	}

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{extism.WasmData{Data: jq}},
		Config: map[string]string{
			"jq.filter": "del(.email)",
		},
	}
	fn, err := json.Marshal(manifest)
	require.NoError(t, err)

	bucket := walrus.NewBucket("bucketname")
	bucket.Add("evacchi", 0, &User{UserId: "evacchi", Email: "edoardo@example.com"})
	bucket.Add("someone", 0, &User{UserId: "someone", Email: "someone@example.com"})

	bucket.PutDDoc(context.Background(), "wasm", &sgbucket.DesignDoc{
		Views: sgbucket.ViewMap{
			"jq": sgbucket.ViewDef{
				Map: string(fn),
			},
		},
	})

	result, err := bucket.View(context.Background(), "wasm", "jq", nil)
	if err != nil {
		panic(err)
	}

	for _, r := range result.Rows {
		u := User{}
		json.Unmarshal([]byte(r.Value.(string)), &u)
		require.NotEmpty(t, u.UserId)
		require.Empty(t, u.Email)
	}

}
