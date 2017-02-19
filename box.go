package main

import (
    "fmt"
    "io"
    ol "github.com/ossrs/go-oryx-lib/logger"
    "encoding/binary"
)

type Box interface {
    Basic() *Mp4Box
    NbHeader() int
    DecodeHeader(r io.Reader) (err error)
}

type Mp4Box struct {
    // The size is the entire size of the box, including the size and type header, fields,
    // and all contained boxes. This facilitates general parsing of the file.
    //
    // if size is 1 then the actual size is in the field largesize;
    // if size is 0, then this box is the last one in the file, and its contents
    // extend to the end of the file (normally only used for a Media Data Box)
    SmallSize uint32
    LargeSize uint64

    // For box 'uuid'.
    UserType  [16]uint8

    // The position at buffer to start demux the box.
    StartPos  int

    // identifies the box type; standard boxes use a compact type, which is normally four printable
    // characters, to permit ease of identification, and is shown so in the boxes below. User extensions use
    // an extended type; in this case, the type field is set to ‘uuid’.
    BoxType   uint32

    Boxes     []Box
    UsedSize  uint64
}

func NewMp4Box() *Mp4Box {
    v := &Mp4Box{}
    v.BoxType = SrsMp4BoxTypeForbidden
    v.Boxes = []Box{}
    v.UserType = [16]uint8{}
    return v
}

// Box type helper.
func (v *Mp4Box) isFtyp() bool {
    return v.BoxType == SrsMp4BoxTypeFTYP
}
func (v *Mp4Box) isMoov() bool {
    return v.BoxType == SrsMp4BoxTypeMOOV
}
func (v *Mp4Box) isMdat() bool {
    return v.BoxType == SrsMp4BoxTypeMDAT
}

// Get the size of box, whatever small or large size.
func (v *Mp4Box) sz() uint64 {
    if v.SmallSize == SRS_MP4_USE_LARGE_SIZE {
        return v.LargeSize
    }
    return uint64(v.SmallSize)
}

func (v *Mp4Box) left() uint64 {
    ol.T(nil, "left:", v.sz(), v.UsedSize)
    return v.sz() - v.UsedSize
}

// Get the contained box of specific type.
// @return The first matched box.
func (v *Mp4Box) get(bt uint32) (error, Box) {
    for _, box := range v.Boxes {
        if box.Basic().BoxType == bt {
            return nil, box
        }
    }
    return fmt.Errorf("can't find bt:%v in boxes", bt), nil
}


// Remove the contained box of specified type.
// @return The removed count.
func (v *Mp4Box) remove(bt uint32) (nbRemoved int) {
    for k, box := range v.Boxes {
        if box.Basic().BoxType == bt {
            v.Boxes = append(v.Boxes[:k], v.Boxes[k+1:]...)
            nbRemoved ++
        }
    }
    return
}

func (v *Mp4Box) NbHeader() int {
    size := 8
    if v.SmallSize == SRS_MP4_USE_LARGE_SIZE {
        size += 8
    }
    if v.BoxType == SrsMp4BoxTypeUUID {
        size += 16
    }
    return size
}

func (v *Mp4Box) decodeHeader(r io.Reader) (err error) {
    v.UsedSize = 0

    // Discovery the size and type.
    if err = v.Read(r, &v.SmallSize); err != nil {
        ol.E(nil, fmt.Sprintf("read small size failed, err is %v", err))
        return
    }
    v.UsedSize += uint64DataSize(v.SmallSize)

    if err = v.Read(r, &v.BoxType); err != nil {
        ol.E(nil, fmt.Sprintf("read type failed, err is %v", err))
        return
    }
    v.UsedSize += uint64DataSize(v.BoxType)

    if v.SmallSize == SRS_MP4_USE_LARGE_SIZE {
        if err = v.Read(r, &v.LargeSize); err != nil {
            ol.E(nil, fmt.Sprintf("read large size failed, err is %v", err))
            return
        }
        v.UsedSize += uint64DataSize(v.LargeSize)
    }

    // Only support 31bits size.
    if (v.LargeSize > 0x7fffffff) {
        err = fmt.Errorf("box overflow")
        ol.E(nil, err.Error())
        return
    }

    if v.BoxType == SrsMp4BoxTypeUUID {
        if err = v.Read(r, &v.UserType); err != nil {
            ol.E(nil, fmt.Sprintf("read user type failed, err is %v", err))
            return
        }
        v.UsedSize += uint64DataSize(v.UserType)
    }
    return
}

func (v *Mp4Box) discovery(r io.Reader) (box Box, err error) {
    v.UsedSize = 0

    // Discovery the size and type.
    var largeSize uint64
    var smallSize uint32

    if err = v.Read(r, &smallSize); err != nil {
        ol.E(nil, fmt.Sprintf("read small size failed, err is %v", err))
        return
    }
    v.UsedSize += uint64DataSize(smallSize)

    var bt uint32
    if err = v.Read(r, &bt); err != nil {
        ol.E(nil, fmt.Sprintf("read type failed, err is %v", err))
        return
    }
    v.UsedSize += uint64DataSize(bt)

    if smallSize == SRS_MP4_USE_LARGE_SIZE {
        if err = v.Read(r, &largeSize); err != nil {
            ol.E(nil, fmt.Sprintf("read large size failed, err is %v", err))
            return
        }
        v.UsedSize += uint64DataSize(largeSize)
    }

    // Only support 31bits size.
    if (largeSize > 0x7fffffff) {
        err = fmt.Errorf("box overflow")
        ol.E(nil, err.Error())
        return
    }

    ol.T(nil, fmt.Sprintf("discovery small size=%v, large size=%v, bt=%x", smallSize, largeSize, bt))
    switch bt {
    case SrsMp4BoxTypeFTYP:
        box = NewMp4FileTypeBox()
    case SrsMp4BoxTypeMOOV:
        box = NewMp4MovieBox()
    case SrsMp4BoxTypeMVHD:
        box = NewMp4MovieHeaderBox()
    case SrsMp4BoxTypeTKHD:
        box = NewMp4TrackHeaderBox()
    default:
        box = NewMp4FreeSpaceBox()
    }

    box.Basic().BoxType = bt
    box.Basic().SmallSize = smallSize
    box.Basic().LargeSize = largeSize
    box.Basic().UsedSize = v.UsedSize
    return
}

func (v *Mp4Box) DecodeBoxes(r io.Reader) (err error) {
    // read left space
    left := v.left()
    ol.T(nil, fmt.Sprintf("after decode header, left space:%v", left))
    for {
        if left <= 0 {
            break
        }

        var box Box
        if box, err = v.discovery(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 discovery contained box failed, err is %v", err))
            return
        }
        ol.T(nil, fmt.Sprintf("discvery a new box:%+v", box))

        if err = box.DecodeHeader(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box header failed, err is %v", err))
            return
        }
        if err = box.Basic().DecodeBoxes(r); err != nil {
            ol.E(nil, fmt.Sprintf("mp4 decode contained box boxes failed, err is %v", err))
            return
        }
        v.Boxes = append(v.Boxes, box)

        left -= box.Basic().sz()
    }
    return
}

func (v *Mp4Box) Skip(r io.Reader, num uint64) {
    data := make([]uint8, num)
    v.Read(r, data)
    v.UsedSize += num
    ol.T(nil, fmt.Sprintf("skip %v bytes", num))
}

func (v *Mp4Box) Read(r io.Reader, data interface{}) (err error) {
    if err = binary.Read(r, binary.BigEndian, data); err != nil {
        return 
    }
    v.UsedSize += uint64DataSize(data)
    return 
}

type Mp4FreeSpaceBox struct {
    Mp4Box
    needSkip int
}

func NewMp4FreeSpaceBox() *Mp4FreeSpaceBox {
    v := &Mp4FreeSpaceBox{
    }
    return v
}

func (v *Mp4FreeSpaceBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FreeSpaceBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + v.needSkip
}

func (v *Mp4FreeSpaceBox) DecodeHeader(r io.Reader) (err error) {
    v.needSkip = int(v.left())
    v.Skip(r, v.left())
    return
}

// ftyp box
type Mp4FileTypeBox struct {
    Mp4Box
    majorBrand uint32
    minorVersion uint32
    compatibleBrands []uint32
}

func NewMp4FileTypeBox() *Mp4FileTypeBox {
    v := &Mp4FileTypeBox{
        majorBrand: SrsMp4BoxBrandForbidden,
        minorVersion: 0,
        compatibleBrands: []uint32{},
    }
    return v
}

func (v *Mp4FileTypeBox) setCompatibleBrands(b0, b1, b2, b3 uint32) {
    v.compatibleBrands = append(v.compatibleBrands, []uint32{b0, b1, b2, b3}...)
}

func (v *Mp4FileTypeBox) DecodeHeader(r io.Reader) (err error) {
    /*if err = v.Mp4Box.DecodeHeader(r); err != nil {
        return err
    }*/

    ol.T(nil, fmt.Sprintf("decode ftyp box, usedSize=%v", v.UsedSize))
    if err = v.Read(r, &v.majorBrand); err != nil {
        ol.E(nil, fmt.Sprintf("read major brand failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.minorVersion); err != nil {
        ol.E(nil, fmt.Sprintf("read minor version failed, err is %v", err))
        return
    }

    // Compatible brands to the end of the box.
    left := v.Mp4Box.left()
    if (left > 0) {
        for i := 0; i < int(left) / 4; i ++ {
            var brand uint32
            if err = v.Read(r, &brand); err != nil {
                ol.E(nil, fmt.Sprintf("read brand failed, err is %v", err))
                return
            }
            v.compatibleBrands = append(v.compatibleBrands, brand)
        }
    }
    return
}

func (v *Mp4FileTypeBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FileTypeBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + 8 + len(v.compatibleBrands) * 4
}

/**
 * 8.2.1 Movie Box (moov)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 30
 * The metadata for a presentation is stored in the single Movie Box which occurs at the top-level of a file.
 * Normally this box is close to the beginning or end of the file, though this is not required.
 */
// moov box
type Mp4MovieBox struct {
    Mp4Box
}

func NewMp4MovieBox() *Mp4MovieBox {
    v := &Mp4MovieBox{}
    return v
}

// Get the header of moov.
func (v *Mp4MovieBox) Mvhd() *Mp4MovieHeaderBox {
    return nil
}

func (v *Mp4MovieBox) AddTrack() {

}

// Get the number of video tracks
func (v *Mp4MovieBox) NbVideoTracks() int {
    return 0
}

// Get the number of audio tracks
func (v *Mp4MovieBox) NbSoundTracks() int {
    return 0
}

func (v *Mp4MovieBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4MovieBox) NbHeader() int {
    return v.Mp4Box.NbHeader()
}

func (v *Mp4MovieBox) DecodeHeader(r io.Reader) (err error) {
    return
}

/**
 * 4.2 Object Structure
 * ISO_IEC_14496-12-base-format-2012.pdf, page 17
 */
type Mp4FullBox struct {
    Mp4Box
    Version uint8
    Flags uint32
}

func (v *Mp4FullBox) Basic() *Mp4Box {
    return &v.Mp4Box
}

func (v *Mp4FullBox) NbHeader() int {
    return v.Mp4Box.NbHeader() + 1 + 3
}

func (v *Mp4FullBox) DecodeHeader(r io.Reader) (err error) {
    /*if err = v.Mp4Box.DecodeHeader(r); err != nil {
        return
    }*/

    if err = v.Read(r, &v.Flags); err != nil {
        ol.E(nil, fmt.Sprintf("read moov flags failed, err is %v", err))
        return
    }

    v.Version = uint8((v.Flags >> 24) & 0xff)
    v.Flags = v.Flags & 0x00ffffff

    return
}

/**
 * 8.2.2 Movie Header Box (mvhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 31
 */
type Mp4MovieHeaderBox struct {
    Mp4FullBox
    // an integer that declares the creation time of the presentation (in seconds since
    // midnight, Jan. 1, 1904, in UTC time)
    CreateTime uint64
    // an integer that declares the most recent time the presentation was modified (in
    // seconds since midnight, Jan. 1, 1904, in UTC time)
    ModTime uint64
    // an integer that specifies the time-scale for the entire presentation; this is the number of
    // time units that pass in one second. For example, a time coordinate system that measures time in
    // sixtieths of a second has a time scale of 60.
    TimeScale uint32
    // an integer that declares length of the presentation (in the indicated timescale). This property
    // is derived from the presentation’s tracks: the value of this field corresponds to the duration of the
    // longest track in the presentation. If the duration cannot be determined then duration is set to all 1s.
    DurationInTbn uint64
    // a fixed point 16.16 number that indicates the preferred rate to play the presentation; 1.0
    // (0x00010000) is normal forward playback
    Rate uint32
    // a fixed point 8.8 number that indicates the preferred playback volume. 1.0 (0x0100) is full volume.
    Volume uint16
    Reserved0 uint16
    Reserved1 uint64
    // a transformation matrix for the video; (u,v,w) are restricted here to (0,0,1), hex values (0,0,0x40000000).
    Matrix [9]int32
    PreDefined [6]uint32
    // a non-zero integer that indicates a value to use for the track ID of the next track to be
    // added to this presentation. Zero is not a valid track ID value. The value of next_track_ID shall be
    // larger than the largest track-ID in use. If this value is equal to all 1s (32-bit maxint), and a new media
    // track is to be added, then a search must be made in the file for an unused track identifier.
    NextTrackId uint32
}

func NewMp4MovieHeaderBox() *Mp4MovieHeaderBox {
    v := &Mp4MovieHeaderBox{
        Matrix: [9]int32{},
        PreDefined: [6]uint32{},
    }
    return v
}

// Get the duration in ms
func (v *Mp4MovieHeaderBox) Duration() uint64 {
    return 0
}

func (v *Mp4MovieHeaderBox) Basic() *Mp4Box {
    return &v.Mp4FullBox.Mp4Box
}

func (v *Mp4MovieHeaderBox) NbHeader() int {
    return 0
}

func (v *Mp4MovieHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if v.Version == 1 {
        if err = v.Read(r, &v.CreateTime); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd create time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.ModTime); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd mod time failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd time scale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &v.DurationInTbn); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd duration failed, err is %v", err))
            return
        }
    } else {
        var tmp uint32
        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd create time failed, err is %v", err))
            return
        }
        v.CreateTime = uint64(tmp)

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd mod time failed, err is %v", err))
            return
        }
        v.ModTime = uint64(tmp)

        if err = v.Read(r, &v.TimeScale); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd time scale failed, err is %v", err))
            return
        }

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("read mvhd duration failed, err is %v", err))
            return
        }
        v.DurationInTbn = uint64(tmp)
    }

    if err = v.Read(r, &v.Rate); err != nil {
        ol.E(nil, fmt.Sprintf("read mvhd rate failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Volume); err != nil {
        ol.E(nil, fmt.Sprintf("read mvhd volume failed, err is %v", err))
        return
    }

    v.Skip(r, v.left())

    return
}

/**
 * 8.3.1 Track Box (trak)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 32
 * This is a container box for a single track of a presentation. A presentation consists of one or more tracks.
 * Each track is independent of the other tracks in the presentation and carries its own temporal and spatial
 * information. Each track will contain its associated Media Box.
 */
type Mp4TrackBox struct {
    Mp4Box
    TrackType uint8
}

/**
 * 8.3.2 Track Header Box (tkhd)
 * ISO_IEC_14496-12-base-format-2012.pdf, page 32
 */
type Mp4TrackHeaderBox struct {
    Mp4FullBox
    CreateTime uint64
    ModTime uint64
    TrackId uint32
    Reserved0 uint32
    Duration uint64
    Reserved1 uint64
    Layer int16
    AlternateGroup int16
    Volume int16
    Reserved2 uint16
    Matrix [9]int32
    Width int32
    Height int32
}

func NewMp4TrackHeaderBox() *Mp4TrackHeaderBox {
    v := &Mp4TrackHeaderBox{
        Matrix: [9]int32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000},
    }
    v.Flags = 0x03
    return v
}

func (v *Mp4TrackHeaderBox) NbHeader() int {
    return 0
}

func (v *Mp4TrackHeaderBox) DecodeHeader(r io.Reader) (err error) {
    if err = v.Mp4FullBox.DecodeHeader(r); err != nil {
        return
    }

    if v.Version == 1 {
        if err = v.Read(r, &v.CreateTime); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read create time failed, err is %v", err))
            return 
        }
        
        if err = v.Read(r, &v.ModTime); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read mod time failed, err is %v", err))
            return
        }
        
        if err = v.Read(r, &v.TrackId); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read track id failed, err is %v", err))
            return
        }
        
        v.Skip(r, uint64(4))
        
        if err = v.Read(r, &v.Duration); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read duration failed, err is %v", err))
            return
        }
    } else {
        var tmp uint32
        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read create time failed, err is %v", err))
            return
        }
        v.CreateTime = uint64(tmp)

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd mod time failed, err is %v", err))
            return
        }
        v.ModTime = uint64(tmp)
        
        if err = v.Read(r, &v.TrackId); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read track id failed, err is %v", err))
            return
        }

        v.Skip(r, uint64(4))

        if err = v.Read(r, &tmp); err != nil {
            ol.E(nil, fmt.Sprintf("tkhd read duration failed, err is %v", err))
            return
        }
        v.Duration = uint64(tmp)
    }
    
    v.Skip(r, uint64(8))
    if err = v.Read(r, &v.Layer); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd layer failed, err is %v", err))
        return 
    }
    
    if err = v.Read(r, &v.AlternateGroup); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd alternate froup failed, err is %v", err))
        return 
    }
    
    if err = v.Read(r, &v.Volume); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd volume failed, err is %v", err))
        return
    }

    v.Skip(r, uint64(2))

    for i := 0; i < len(v.Matrix); i ++ {
        if err = v.Read(r, &v.Matrix[i]); err != nil {
            ol.E(nil, fmt.Sprintf("read tkhd matrix %d failed, err is %v", i, err))
            return
        }
    }

    if err = v.Read(r, &v.Width); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd width failed, err is %v", err))
        return
    }

    if err = v.Read(r, &v.Height); err != nil {
        ol.E(nil, fmt.Sprintf("read tkhd height failed, err is %v", err))
        return
    }
    return
}
