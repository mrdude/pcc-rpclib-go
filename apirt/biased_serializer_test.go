package apirt

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/barkimedes/go-deepcopy"
	"github.com/go-test/deep"
	"github.com/mrdude/pcc-common"
)

// these objects are copied to prevent import cycles with other parts of the pcc codebase

type iopType string

func (v iopType) IsValid() bool {
	switch v {
	case iopTypeREAD:
		fallthrough
	case iopTypeWRITE:
		return true
	default:
		return false
	}
}

func (v iopType) Validate() error {
	if !v.IsValid() {
		return fmt.Errorf("invalid value '%s' (must be one of READ,WRITE)", string(v))
	}
	return nil
}

const (
	iopTypeREAD  iopType = "READ"
	iopTypeWRITE iopType = "WRITE"
)

// performBlockIOInput: a block I/O request
type performBlockIOInput struct {
	RequestId int64      `json:"requestId"` // this ID is echoed back in the response, and is used to match requests to responses
	DiskId    string     `json:"diskId"`    // read/write: the disk to direct this IO request to
	OpType    iopType    `json:"opType"`    // read/write: the type of I/O operation
	Offset    int64      `json:"offset"`    // read/write: the offset to read/write from
	Count     int32      `json:"count"`     // read: the number of bytes to read
	Data      ByteString `json:"data"`      // write: the data to write
}

// performBlockIOOutput: a block I/O request
type performBlockIOOutput struct {
	RequestId int64      `json:"requestId"`
	Err       string     `json:"err"`  // read/write: a description of any error that occurred
	Data      ByteString `json:"data"` // read: the data that was read
}

type createDiskOutput struct {
	Disk virtualDisk `json:"disk"`
}

type listDisksOutput struct {
	Disks     []virtualDisk `json:"disks"`
	NextToken *string       `json:"nextToken,omitempty"`
}

type pullDiskListingOutput struct {
	Disks []*virtualDisk `json:"mountedDisks"`
}

type virtualDisk struct {
	Id              string `json:"id"`              // the ID of the disk
	Size            int64  `json:"size"`            // the size of the disk, in bytes
	OwningBackend   string `json:"owningBackend"`   // the full name of the machine whose backend owns this disk
	MountedFrontend string `json:"mountedFrontend"` // the full name of the machine whose frontend has mounted this disk. blank if the disk is unmounted.
}

//

type serializerTestCase struct {
	Name string
	Obj  any
}

var serializerTestCases = []*serializerTestCase{
	{
		Name: "Input",
		Obj: &performBlockIOInput{
			RequestId: 1,
			DiskId:    "disk-id",
			OpType:    iopTypeREAD,
			Offset:    2 * gb,
			Count:     int32(4 * kb),
			Data:      NewByteStringFromBytes(pcommon.GenerateCryptoRandomBytes(int(4 * kb))),
		},
	},
	{
		Name: "Output",
		Obj: &performBlockIOOutput{
			RequestId: 2,
			Err:       "test string",
			Data:      NewByteStringFromBytes(pcommon.GenerateCryptoRandomBytes(int(4 * kb))),
		},
	},
	{
		Name: "CreateDiskOutput",
		Obj: &createDiskOutput{
			Disk: virtualDisk{
				Id:              "test-id",
				Size:            256 * gb,
				OwningBackend:   "test-backend",
				MountedFrontend: "test-frontend",
			},
		},
	},
	{
		Name: "ListDisks",
		Obj: &listDisksOutput{
			Disks: []virtualDisk{
				virtualDisk{
					Id:              "test-id",
					Size:            256 * gb,
					OwningBackend:   "test-backend",
					MountedFrontend: "test-frontend",
				},
				virtualDisk{
					Id:              "test-id2",
					Size:            512 * gb,
					OwningBackend:   "test-backend",
					MountedFrontend: "test-frontend",
				},
			},
			NextToken: new("hi"),
		},
	},
	{
		Name: "ListOfPointers",
		Obj: &pullDiskListingOutput{
			Disks: []*virtualDisk{
				&virtualDisk{
					Id:              "test-id",
					Size:            256 * gb,
					OwningBackend:   "test-backend",
					MountedFrontend: "test-frontend",
				},
				&virtualDisk{
					Id:              "test-id2",
					Size:            512 * gb,
					OwningBackend:   "test-backend",
					MountedFrontend: "test-frontend",
				},
			},
		},
	},
}

func TestBiasedSerializer(t *testing.T) {
	for i := range serializerTestCases {
		for j := range serializers {
			tc := serializerTestCases[i]
			ser := serializers[j]
			name := fmt.Sprintf("TC-%s-%s", tc.Name, ser.Name())
			t.Run(name, func(t *testing.T) {
				testSerializer(t, ser, tc.Obj)
			})
		}
	}
}

func testSerializer(t *testing.T, ser *Serializer, obj any) {
	// create a copy of the object
	expected := deepcopy.MustAnything(obj)

	// serialize it
	b, err := ser.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}

	// deserialize it
	actual := reflect.New(reflect.TypeOf(obj).Elem()).Interface()
	err = ser.Unmarshal(b, actual)
	if err != nil {
		t.Fatal(err)
	}

	// test for equality
	if diffs := deep.Equal(actual, expected); len(diffs) > 0 {
		// TODO pretty-print this
		t.Fatalf("Values do not match (%q)", diffs)
	}
}

func BenchmarkBiasSerializers(b *testing.B) {
	for i := range serializerTestCases {
		for j := range serializers {
			tc := serializerTestCases[i]
			ser := serializers[j]
			name := fmt.Sprintf("TC-%s-%s", tc.Name, ser.Name())
			b.Run(name, func(b *testing.B) {
				benchSerializer(b, ser, tc.Obj)
			})
		}
	}
}

func benchSerializer(b *testing.B, ser *Serializer, obj any) {
	actual := reflect.New(reflect.TypeOf(obj).Elem()).Interface()

	var serBytes []byte
	var err error
	for i := 0; i < b.N; i++ {
		// serialize it
		serBytes, err = ser.Marshal(obj)
		if err != nil {
			b.Fatal(err)
		}

		// deserialize it
		err = ser.Unmarshal(serBytes, actual)
		if err != nil {
			b.Fatal(err)
		}
	}
}
