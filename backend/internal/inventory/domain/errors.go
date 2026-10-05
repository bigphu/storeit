package domain

import "storeit/internal/platform/errs"

// Lỗi nghiệp vụ của inventory; status HTTP đi kèm, handler không phải đổi
var (
	// Một lỗi cho mọi giá trị thuộc tính sai, mỗi key một FieldError (ValidateValues)
	ErrInvalidAttributeValues = errs.Unprocessable("/errors/invalid-attribute-values", "Invalid attribute values")

	// Lọc/sắp theo thuộc tính sai (ResolveAttrQuery): field "attr[i]", "sort" hoặc "type_id"
	ErrInvalidAttributeQuery = errs.Unprocessable("/errors/invalid-attribute-query", "Invalid attribute filter or sort")

	ErrInvalidBulk = errs.Unprocessable("/errors/invalid-bulk", "Invalid selection",
		errs.WithFields(errs.FieldError{Field: "items", Detail: "choose between 1 and 200 assets"}))

	ErrInvalidOrder = errs.Unprocessable("/errors/invalid-order", "Order doesn't match",
		errs.WithDetail("Send every active attribute or option exactly once. Someone may have just added or removed one; reload and try again."))

	ErrInvalidTag = errs.Unprocessable("/errors/invalid-tag", "Invalid asset tag",
		errs.WithFields(errs.FieldError{Field: "tag", Detail: "must be 1-64 characters: A-Z, 0-9, '.', '_' or '-', starting with a letter or digit"}))
	ErrInvalidTypeCode = errs.Unprocessable("/errors/invalid-type-code", "Invalid asset type code",
		errs.WithFields(errs.FieldError{Field: "code", Detail: "must be 1-32 characters: A-Z, 0-9, '_' or '-'"}))
	ErrInvalidAttributeKey = errs.Unprocessable("/errors/invalid-attribute-key", "Invalid attribute key",
		errs.WithFields(errs.FieldError{Field: "key", Detail: "must start with a-z and contain only a-z, 0-9 and '_' (max 32)"}))
	ErrInvalidLabel = errs.Unprocessable("/errors/invalid-label", "Invalid name or label",
		errs.WithDetail("Names and labels must not be blank, too long or contain control characters."))
	ErrInvalidUnit = errs.Unprocessable("/errors/invalid-unit", "Invalid unit",
		errs.WithFields(errs.FieldError{Field: "unit", Detail: "only number attributes have a unit, up to 16 characters"}))
	ErrInvalidDataType = errs.Unprocessable("/errors/invalid-data-type", "Invalid data type",
		errs.WithFields(errs.FieldError{Field: "data_type", Detail: "must be text, number, date, boolean or select"}))
	ErrInvalidStatusKind = errs.Unprocessable("/errors/invalid-status-kind", "Invalid status kind",
		errs.WithFields(errs.FieldError{Field: "kind", Detail: "must be available, in_use, unavailable or retired"}))
	ErrTypeArchived = errs.Unprocessable("/errors/asset-type-archived", "Asset type archived",
		errs.WithDetail("Archived asset types cannot be chosen for an asset."))
	ErrStatusArchived = errs.Unprocessable("/errors/status-archived", "Status archived",
		errs.WithDetail("Archived statuses cannot be chosen for an asset."))
	ErrRetiredStatus = errs.Unprocessable("/errors/retired-status", "Use retire",
		errs.WithDetail("A retired status is set by retiring the asset, not by editing it."))
	ErrNotSelectAttribute = errs.Unprocessable("/errors/not-select-attribute", "Not a choice attribute",
		errs.WithDetail("Only select attributes have options."))

	ErrTagTaken            = errs.Conflict("/errors/tag-taken", "Asset tag already in use")
	ErrTypeCodeTaken       = errs.Conflict("/errors/type-code-taken", "Asset type code already in use")
	ErrTypeNameTaken       = errs.Conflict("/errors/type-name-taken", "Asset type name already in use")
	ErrAttributeKeyTaken   = errs.Conflict("/errors/attribute-key-taken", "Attribute key already used in this type")
	ErrAttributeLabelTaken = errs.Conflict("/errors/attribute-label-taken", "Attribute label already used in this type")
	ErrOptionLabelTaken    = errs.Conflict("/errors/option-label-taken", "Option label already used in this attribute")
	ErrStatusNameTaken     = errs.Conflict("/errors/status-name-taken", "Status name already in use")
	ErrAssetChanged        = errs.Conflict("/errors/asset-changed", "Asset changed",
		errs.WithDetail("The asset was changed by someone else. Reload it and try again."))
	ErrAssetTypeChanged = errs.Conflict("/errors/asset-type-changed", "Asset type changed",
		errs.WithDetail("The asset type was changed by someone else. Reload it and try again."))
	ErrAssetRetired = errs.Conflict("/errors/asset-retired", "Asset retired",
		errs.WithDetail("Restore the asset before editing it."))
	ErrAttributeInUse = errs.Conflict("/errors/attribute-in-use", "Attribute in use",
		errs.WithDetail("The data type or unit cannot change while assets have values for this attribute."))
	ErrSystemType = errs.Conflict("/errors/system-type", "System asset type",
		errs.WithDetail("The General asset type cannot be archived."))
	ErrSystemStatus = errs.Conflict("/errors/system-status", "System status",
		errs.WithDetail("Built-in statuses cannot be archived or change kind."))
	ErrStatusIsDefault = errs.Conflict("/errors/status-is-default", "Default status",
		errs.WithDetail("Make another status the default of this kind before archiving it."))

	ErrAssetNotFound     = errs.NotFound("/errors/asset-not-found", "Asset not found")
	ErrTypeNotFound      = errs.NotFound("/errors/asset-type-not-found", "Asset type not found")
	ErrAttributeNotFound = errs.NotFound("/errors/attribute-not-found", "Attribute not found")
	ErrOptionNotFound    = errs.NotFound("/errors/option-not-found", "Option not found")
	ErrStatusNotFound    = errs.NotFound("/errors/status-not-found", "Status not found")
)
