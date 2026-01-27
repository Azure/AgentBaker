package toolkit

func Map[OriginalElementType, TransformedElementType any](ts []OriginalElementType, f func(OriginalElementType) TransformedElementType) []TransformedElementType {
	us := make([]TransformedElementType, len(ts))
	for currentIndex := range ts {
		us[currentIndex] = f(ts[currentIndex])
	}
	return us
}

func Filter[ElementType any](ss []ElementType, test func(ElementType) bool) (ret []ElementType) {
	for _, item := range ss {
		if test(item) {
			ret = append(ret, item)
		}
	}
	return
}

func Reduce[ElementType, ReturnType any](s []ElementType, initialValue ReturnType, f func(sum ReturnType, next ElementType) ReturnType) ReturnType {
	sum := initialValue
	for _, next := range s {
		sum = f(sum, next)
	}
	return sum
}
