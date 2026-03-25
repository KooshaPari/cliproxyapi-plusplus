// Package interfaces provides type aliases for backwards compatibility with translator functions.
// It defines common interface types used throughout the CLI Proxy API for request and response
// transformation operations, maintaining compatibility with the SDK translator package.
package interfaces

<<<<<<< HEAD
import sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
=======
import sdktranslator "github.com/kooshapari/cliproxyapi-plusplus/v6/sdk/translator"
>>>>>>> origin/main

// Backwards compatible aliases for translator function types.
type TranslateRequestFunc = sdktranslator.RequestTransform

type TranslateResponseFunc = sdktranslator.ResponseStreamTransform

type TranslateResponseNonStreamFunc = sdktranslator.ResponseNonStreamTransform

type TranslateResponse = sdktranslator.ResponseTransform
