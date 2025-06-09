package chaindb

import "slices"

// NextPrefix returns the next prefix in lexicographical order.
//
// It increments the last byte of the prefix that is not 0xFF, and sets all
// subsequent bytes to 0x00. If all bytes are 0xFF, it appends a new byte with
// value 0x00 to the end of the prefix.
//
// For example, if the input is [0x01, 0x02, 0xFF], the output will be
// [0x01, 0x03, 0x00].
//
// If the input is [0xFF, 0xFF], the output will be
// [0xFF, 0xFF, 0x00].
//
// If the input is [0x01, 0x02], the output will be
// [0x01, 0x03].
//
// If the input is empty, it returns [0x00].
//
// If the input is nil, it returns [0x00].
func NextPrefix(prefix []byte) []byte {
	np := slices.Clone(prefix)
	for i := len(np) - 1; i >= 0; i-- {
		if np[i] < 0xFF {
			np[i]++
			return np
		}
		np[i] = 0
	}
	return append(np, 0x00)
}

// UpperBound returns the upper bound for the given prefix.
// The difference from NextPrefix is that it returns the first key that
// does not match the prefix, rather than the next prefix in lexicographical order.
// It increments the last byte of the prefix that is not 0xFF, and sets all
// subsequent bytes to 0xFF. If all bytes are 0xFF, it returns an empty slice.
// For example, if the input is [0x01, 0x02, 0xFF], the output will be
// [0x01, 0x03, 0x00].
// If the input is [0xFF, 0xFF], the output will be an empty slice.
// If the input is [0x01, 0x02], the output will be
// [0x01, 0x03, 0xFF].
// If the input is empty, it returns an empty slice.
// If the input is nil, it returns an empty slice.
//
// Note: This function is used to determine the upper bound for iterators, so
// it is important that it returns the first key that does not match the prefix.
// It is not the same as NextPrefix, which returns the next prefix in lexicographical order.
func UpperBound(prefix []byte) (limit []byte) {
	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c == 0xFF {
			continue
		}
		limit = make([]byte, i+1)
		copy(limit, prefix)
		limit[i] = c + 1
		break
	}
	return limit
}
