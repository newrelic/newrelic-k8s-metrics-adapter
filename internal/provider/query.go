// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const limitClause = " limit "

func addLimit(query string) string {
	query = fmt.Sprintf("%s limit 1", query)

	return query
}

func addClusterFilter(clusterName string, query string) string {
	return fmt.Sprintf("%s where clusterName='%s'", query, clusterName)
}

func addMatchFilter(match labels.Selector, query string) string {
	requirements, ok := match.Requirements()
	if !ok || len(requirements) == 0 {
		return query
	}

	whereClause := "where"

	for index, r := range requirements {
		key := r.Key()

		switch r.Operator() {
		case selection.Equals, selection.DoubleEquals, selection.GreaterThan, selection.LessThan, selection.NotEquals:
			whereClause = buildSimpleCondition(whereClause, key, r.Operator(), r.Values().List()[0])

		case selection.In, selection.NotIn:
			whereClause = buildINClause(whereClause, key, r.Operator(), r.Values().List())

		case selection.DoesNotExist, selection.Exists:
			whereClause = fmt.Sprintf("%s %s %s", whereClause, key, transform[r.Operator()])
		}

		if index != len(requirements)-1 {
			whereClause = fmt.Sprintf("%s and", whereClause)
		}
	}

	return fmt.Sprintf("%s %s", query, whereClause)
}

func buildINClause(whereClause string, key string, operator selection.Operator, values []string) string {
	inClause := "("

	for index, v := range values {
		//nolint: gomnd
		if _, errNoNumber := strconv.ParseFloat(v, 64); errNoNumber != nil {
			inClause = fmt.Sprintf("%s'%s'", inClause, v)
		} else {
			inClause = fmt.Sprintf("%s%s", inClause, v)
		}

		if index != len(values)-1 {
			inClause = fmt.Sprintf("%s, ", inClause)
		}
	}

	inClause = fmt.Sprintf("%s)", inClause)

	return fmt.Sprintf("%s %s %s %s", whereClause, key, transform[operator], inClause)
}

func buildSimpleCondition(whereClause string, key string, operator selection.Operator, value string) string {
	// Note that this is a simplification since it is possible that we have a valid number, but we want it as a string.
	// Es: systemMemoryBytes is a number and reported as a string
	// nolint: gomnd
	if _, errNoNumber := strconv.ParseFloat(value, 64); errNoNumber != nil {
		return fmt.Sprintf("%s %s %s '%s'", whereClause, key, transform[operator], value)
	}

	return fmt.Sprintf("%s %s %s %s", whereClause, key, transform[operator], value)
}

// nolint: gochecknoglobals
var transform = map[selection.Operator]string{
	selection.NotEquals:    "!=",
	selection.Equals:       "=",
	selection.GreaterThan:  ">",
	selection.DoubleEquals: "=",
	selection.LessThan:     ",",
	selection.Exists:       "IS NOT NULL",
	selection.DoesNotExist: "IS NULL",
	selection.In:           "IN",
	selection.NotIn:        "NOT IN",
}
