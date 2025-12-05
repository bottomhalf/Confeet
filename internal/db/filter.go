package db

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FilterBuilder helps build MongoDB filters fluently
type FilterBuilder struct {
	filter bson.M
}

// NewFilter creates a new FilterBuilder
func NewFilter() *FilterBuilder {
	return &FilterBuilder{filter: bson.M{}}
}

// Eq adds an equality condition
func (f *FilterBuilder) Eq(field string, value interface{}) *FilterBuilder {
	f.filter[field] = value
	return f
}

// Ne adds a not-equal condition
func (f *FilterBuilder) Ne(field string, value interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$ne": value}
	return f
}

// Gt adds a greater-than condition
func (f *FilterBuilder) Gt(field string, value interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$gt": value}
	return f
}

// Gte adds a greater-than-or-equal condition
func (f *FilterBuilder) Gte(field string, value interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$gte": value}
	return f
}

// Lt adds a less-than condition
func (f *FilterBuilder) Lt(field string, value interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$lt": value}
	return f
}

// Lte adds a less-than-or-equal condition
func (f *FilterBuilder) Lte(field string, value interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$lte": value}
	return f
}

// In adds an $in condition (value in array)
func (f *FilterBuilder) In(field string, values interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$in": values}
	return f
}

// NotIn adds a $nin condition (value not in array)
func (f *FilterBuilder) NotIn(field string, values interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$nin": values}
	return f
}

// Regex adds a regex pattern match
func (f *FilterBuilder) Regex(field string, pattern string, options string) *FilterBuilder {
	f.filter[field] = bson.M{"$regex": pattern, "$options": options}
	return f
}

// Contains adds a case-insensitive contains search
func (f *FilterBuilder) Contains(field string, value string) *FilterBuilder {
	f.filter[field] = bson.M{"$regex": value, "$options": "i"}
	return f
}

// Exists checks if field exists
func (f *FilterBuilder) Exists(field string, exists bool) *FilterBuilder {
	f.filter[field] = bson.M{"$exists": exists}
	return f
}

// Between adds a range condition (inclusive)
func (f *FilterBuilder) Between(field string, min, max interface{}) *FilterBuilder {
	f.filter[field] = bson.M{"$gte": min, "$lte": max}
	return f
}

// ObjectID adds an ObjectID filter
func (f *FilterBuilder) ObjectID(field string, id string) *FilterBuilder {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err == nil {
		f.filter[field] = objectID
	}
	return f
}

// And combines multiple filters with AND
func (f *FilterBuilder) And(filters ...bson.M) *FilterBuilder {
	if len(filters) > 0 {
		f.filter["$and"] = filters
	}
	return f
}

// Or combines multiple filters with OR
func (f *FilterBuilder) Or(filters ...bson.M) *FilterBuilder {
	if len(filters) > 0 {
		f.filter["$or"] = filters
	}
	return f
}

// Build returns the final bson.M filter
func (f *FilterBuilder) Build() bson.M {
	return f.filter
}

// Empty returns an empty filter (matches all documents)
func Empty() bson.M {
	return bson.M{}
}
