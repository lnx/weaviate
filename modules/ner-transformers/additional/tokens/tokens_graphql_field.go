//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2021 SeMI Technologies B.V. All rights reserved.
//
//  CONTACT: hello@semi.technology
//

package tokens

import (
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/semi-technologies/weaviate/adapters/handlers/graphql/descriptions"
)

func (p *TokenProvider) additionalTokensField(classname string) *graphql.Field {
	return &graphql.Field{
		Args: graphql.FieldConfigArgument{
			"properties": &graphql.ArgumentConfig{
				Description:  "Properties which contains text",
				Type:         graphql.NewList(graphql.String),
				DefaultValue: nil,
			},
			"certainty": &graphql.ArgumentConfig{
				Description:  descriptions.Certainty,
				Type:         graphql.Float,
				DefaultValue: nil,
			},
			"limit": &graphql.ArgumentConfig{
				Type:         graphql.Int,
				Description:  descriptions.Limit,
				DefaultValue: nil,
			},
		},
		Type: graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
			Name: fmt.Sprintf("%sAdditionalTokens", classname),
			Fields: graphql.Fields{
				"property":      &graphql.Field{Type: graphql.String},
				"entity":        &graphql.Field{Type: graphql.String},
				"certainty":     &graphql.Field{Type: graphql.Float},
				"word":          &graphql.Field{Type: graphql.String},
				"startPosition": &graphql.Field{Type: graphql.Int},
				"endPosition":   &graphql.Field{Type: graphql.Int},
			},
		})),
	}
}
