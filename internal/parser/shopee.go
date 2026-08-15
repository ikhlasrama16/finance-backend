package parser

type shopeeParser struct{}

func (shopeeParser) CanParse(input Input) bool {
	return normalizeText(input.SourceApp) == "shopee" && isShopeeTopUp(input)
}

func (shopeeParser) Parse(Input) (*Result, error) {
	return &Result{Ignore: true, ParseStatus: "IGNORED_SUPPORTING_NOTIFICATION"}, nil
}

func isShopeeTopUp(input Input) bool {
	title := normalizeText(input.Title)
	text := normalizedInput(input)
	if containsAny(title, "top-up completed", "top up completed") {
		return true
	}
	return containsAny(text, "top up request") && containsAny(text, "successful", "success")
}
