package util

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

const (
	// TODO: this is an arbitrary size. lexicons can set more realistic limits,
	// and we should pass those limits through and only fall back to this when
	// undefined.
	MAX_BYTE_ARRAY_SIZE = 128 * 1024 * 1024
)

type LexLink cid.Cid

type jsonLink struct {
	Link string `json:"$type"`
}

// convenience helper
func (ll LexLink) String() string {
	return cid.Cid(ll).String()
}

// convenience helper
func (ll LexLink) Defined() bool {
	return cid.Cid(ll).Defined()
}

func (ll *LexLink) MarshalJSON() ([]byte, error) {
	if ll == nil || !ll.Defined() {
		return nil, xerrors.Errorf("tried to marshal nil or undefined cid-link")
	}
	jl := jsonLink{
		Link: (*cid.Cid)(ll).String(),
	}
	return json.Marshal(jl)
}

func (ll *LexLink) UnmarshalJSON(raw []byte) error {
	var jl jsonLink
	err := json.Unmarshal(raw, &jl)
	if err != nil {
		return xerrors.Errorf("parsing cid-link JSON: %v", err)
	}
	c, err := cid.Decode(jl.Link)
	if err != nil {
		return xerrors.Errorf("parsing cid-link CID: %v", err)
	}
	*ll = LexLink(c)
	return nil
}

func (ll *LexLink) MarshalCBOR(w io.Writer) error {
	if ll == nil || !ll.Defined() {
		return xerrors.Errorf("tried to marshal nil or undefined cid-link")
	}
	cw := cbg.NewCborWriter(w)
	if err := cbg.WriteCid(cw, cid.Cid(*ll)); err != nil {
		return xerrors.Errorf("failed to write cid-link as CBOR: %w", err)
	}
	return nil
}

func (ll *LexLink) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	c, err := cbg.ReadCid(cr)
	if err != nil {
		return xerrors.Errorf("failed to read cid-link from CBOR: %w", err)
	}
	*ll = LexLink(c)
	return nil
}

type LexBytes []byte

type jsonBytes struct {
	Bytes string `json:"$bytes"`
}

func (lb *LexBytes) MarshalJSON() ([]byte, error) {
	if lb == nil {
		return nil, xerrors.Errorf("tried to marshal nil $bytes")
	}
	jb := jsonBytes{
		Bytes: base64.StdEncoding.EncodeToString([]byte(*lb)),
	}
	return json.Marshal(jb)
}

func (lb *LexBytes) UnmarshalJSON(raw []byte) error {
	var jb jsonBytes
	err := json.Unmarshal(raw, &jb)
	if err != nil {
		return xerrors.Errorf("parsing $bytes JSON: %v", err)
	}
	out, err := base64.StdEncoding.DecodeString(jb.Bytes)
	if err != nil {
		return xerrors.Errorf("parsing $bytes base64: %v", err)
	}
	*lb = LexBytes(out)
	return nil
}

func (lb *LexBytes) MarshalCBOR(w io.Writer) error {
	if lb == nil {
		return xerrors.Errorf("tried to marshal nil or undefined $bytes")
	}
	cw := cbg.NewCborWriter(w)
	if err := cbg.WriteByteArray(cw, ([]byte)(*lb)); err != nil {
		return xerrors.Errorf("failed to write $bytes as CBOR: %w", err)
	}
	return nil
}

func (lb *LexBytes) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	b, err := cbg.ReadByteArray(cr, MAX_BYTE_ARRAY_SIZE)
	if err != nil {
		return xerrors.Errorf("failed to read $bytes from CBOR: %w", err)
	}
	*lb = LexBytes(b)
	return nil
}

// used in schemas, and can represent either a legacy blob or a "new" (lex
// refactor) blob. size=-1 indicates that this is (and should be serialized as)
// a legacy blob (string CID, no size, etc).
type LexBlob struct {
	Ref      LexLink `json:"ref" cborgen:"ref"`
	MimeType string  `json:"mimeType" cborgen:"mimeType"`
	Size     int64   `json:"size" cborgen:"size"`
}

type LegacyBlob struct {
	Cid      string `json:"cid" cborgen:"cid"`
	MimeType string `json:"mimeType" cborgen:"mimeType"`
}

type BlobSchema struct {
	LexiconTypeID string  `json:"$type,omitempty"`
	Ref           LexLink `json:"ref" cborgen:"ref"`
	MimeType      string  `json:"mimeType" cborgen:"mimeType"`
	Size          int64   `json:"size" cborgen:"size"`
}

func (b *LexBlob) MarshalJSON() ([]byte, error) {
	if b.Size < 0 {
		lb := LegacyBlob{
			Cid:      b.Ref.String(),
			MimeType: b.MimeType,
		}
		return json.Marshal(lb)
	} else {
		nb := BlobSchema{
			LexiconTypeID: "blob",
			Ref:           b.Ref,
			MimeType:      b.MimeType,
			Size:          b.Size,
		}
		return json.Marshal(nb)
	}
}

func (b *LexBlob) UnmarshalJSON(raw []byte) error {
	typ, err := TypeExtract(raw)
	if err != nil {
		return xerrors.Errorf("parsing blob: %v", err)
	}

	if typ == "blob" {
		var bs BlobSchema
		err := json.Unmarshal(raw, &bs)
		if err != nil {
			return xerrors.Errorf("parsing blob JSON: %v", err)
		}
		b.Ref = bs.Ref
		b.MimeType = bs.MimeType
		b.Size = bs.Size
		if bs.Size < 0 {
			return xerrors.Errorf("parsing blob: negative size: %d", bs.Size)
		}
	} else {
		var legacy *LegacyBlob
		err := json.Unmarshal(raw, legacy)
		if err != nil {
			return xerrors.Errorf("parsing legacy blob: %v", err)
		}
		refCid, err := cid.Decode(legacy.Cid)
		if err != nil {
			return xerrors.Errorf("parsing CID in legacy blob: %v", err)
		}
		b.Ref = LexLink(refCid)
		b.MimeType = legacy.MimeType
		b.Size = -1
	}
	return nil
}

func (b *LexBlob) MarshalCBOR(w io.Writer) error {
	if b == nil {
		return nil
	}
	if b.Size < 0 {
		lb := LegacyBlob{
			Cid:      b.Ref.String(),
			MimeType: b.MimeType,
		}
		return lb.MarshalCBOR(w)
	} else {
		bs := BlobSchema{
			LexiconTypeID: "blob",
			Ref:           b.Ref,
			MimeType:      b.MimeType,
			Size:          b.Size,
		}
		return bs.MarshalCBOR(w)
	}
}

func (lb *LexBlob) UnmarshalCBOR(r io.Reader) error {
	typ, b, err := CborTypeExtractReader(r)
	if err != nil {
		return xerrors.Errorf("parsing $blob CBOR type: %w", err)
	}
	*lb = LexBlob{}
	if typ == "blob" {
		var bs BlobSchema
		err := bs.UnmarshalCBOR(bytes.NewReader(b))
		if err != nil {
			return xerrors.Errorf("parsing $blob CBOR: %v", err)
		}
		lb.Ref = bs.Ref
		lb.MimeType = bs.MimeType
		lb.Size = bs.Size
		if bs.Size < 0 {
			return xerrors.Errorf("parsing $blob CBOR: negative size: %d", bs.Size)
		}
	} else {
		legacy := LegacyBlob{}
		err := legacy.UnmarshalCBOR(bytes.NewReader(b))
		if err != nil {
			return xerrors.Errorf("parsing legacy blob CBOR: %v", err)
		}
		refCid, err := cid.Decode(legacy.Cid)
		if err != nil {
			return xerrors.Errorf("parsing CID in legacy blob CBOR: %v", err)
		}
		lb.Ref = LexLink(refCid)
		lb.MimeType = legacy.MimeType
		lb.Size = -1
	}

	return nil
}
