package graphql

//go:generate echo ""
//go:generate echo "------> Generating graphql code from graph/graphql/schema"
//go:generate go run ./gqlgen.go
//go:generate echo "------> Generating finished"
