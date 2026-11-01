package engine

import (
	"reflect"
	"testing"
)

func TestNamedPCGReferenceVector(t *testing.T) {
	stream := newNamedStream(42, "arrivals")
	got := []uint32{stream.nextUint32(), stream.nextUint32(), stream.nextUint32()}
	want := []uint32{1148725132, 1651341993, 3015146221}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PCG reference vector changed: %v", got)
	}
}
