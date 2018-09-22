package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/graph-gophers/dataloader"
	"github.com/graphql-go/graphql"
)

type Category struct {
	ID   uint
	Name string
}

type Product struct {
	ID                uint
	Title             string
	ProductCategories []ProductCategory
}

type ProductCategory struct {
	CategoryID uint
	ProductID  uint
}

type Order struct {
	ID         uint
	UserID     uint
	OrderItems []OrderItem
}

type OrderItem struct {
	OrderID   uint
	ProductID uint
}

type User struct {
	ID        uint
	FirstName string
	LastName  string
	Orders    []Order
}

type Client struct {
}

func (c *Client) ListUserOrders(userIDs []uint) ([]Order, error) {
	return []Order{
		Order{
			ID:     200,
			UserID: 1,
			OrderItems: []OrderItem{
				OrderItem{OrderID: 200, ProductID: 100},
			},
		},
	}, nil
}

func (c *Client) ListCategories(categoryIDs []uint) ([]Category, error) {
	var categories []Category
	for _, categoryID := range categoryIDs {
		c := Category{ID: categoryID, Name: fmt.Sprintf("name#%v", categoryID)}
		categories = append(categories, c)
	}
	return categories, nil
}

func (c *Client) ListProducts(productIDs []uint) ([]Product, error) {
	var products []Product
	for _, productID := range productIDs {
		product := Product{
			ID:    productID,
			Title: fmt.Sprintf("product#%v", productID),
			ProductCategories: []ProductCategory{
				ProductCategory{ProductID: productID, CategoryID: 1},
				ProductCategory{ProductID: productID, CategoryID: 2},
				ProductCategory{ProductID: productID, CategoryID: 3},
			},
		}
		products = append(products, product)
	}
	return products, nil
}

type ResolverKey struct {
	Key    string
	Client *Client
}

func (rk *ResolverKey) client() *Client {
	return rk.Client
}

func NewResolverKey(key string, client *Client) *ResolverKey {
	return &ResolverKey{
		Key:    key,
		Client: client,
	}
}

func (rk *ResolverKey) String() string {
	return rk.Key
}

func (rk *ResolverKey) Raw() interface{} {
	return rk.Key
}

var CategoryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Category",
	Fields: graphql.Fields{
		"id":   &graphql.Field{Type: graphql.Int},
		"name": &graphql.Field{Type: graphql.String},
	},
})

var ProductType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Product",
	Fields: graphql.Fields{
		"id":    &graphql.Field{Type: graphql.Int},
		"title": &graphql.Field{Type: graphql.String},
		"categories": &graphql.Field{
			Type: graphql.NewList(CategoryType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var (
					product      = p.Source.(Product)
					v            = p.Context.Value
					loaders      = v("loaders").(map[string]*dataloader.Loader)
					c            = v("client").(*Client)
					thunks       []dataloader.Thunk
					wg           sync.WaitGroup
					handleErrors = func(errors []error) error {
						var errs []string
						for _, e := range errors {
							errs = append(errs, e.Error())
						}
						return fmt.Errorf(strings.Join(errs, "\n"))
					}
				)

				for _, productCategory := range product.ProductCategories {
					id := productCategory.CategoryID
					key := NewResolverKey(fmt.Sprintf("%d", id), c)
					// Here we could use `.LoadMany`
					// like: `categories` Resolve function.
					// But using `.Load` instead to demostrate that
					// queries are batched.
					thunk := loaders["GetCategory"].Load(p.Context, key)
					thunks = append(thunks, thunk)
				}

				type result struct {
					categories []Category
					errs       []error
				}
				ch := make(chan *result, 1)

				go func() {
					var categories []Category
					var errs []error
					for _, thunk := range thunks {
						wg.Add(1)
						go func(t dataloader.Thunk) {
							defer wg.Done()
							r, err := t()
							if err != nil {
								errs = append(errs, err)
								return
							}
							c := r.(Category)
							categories = append(categories, c)
						}(thunk)

					}
					wg.Wait()
					ch <- &result{categories: categories, errs: errs}
				}()

				return func() (interface{}, error) {
					r := <-ch
					if len(r.errs) > 0 {
						return nil, handleErrors(r.errs)
					}
					return r.categories, nil
				}, nil

			},
		},
	},
})

var OrderType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Order",
	Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.Int},
		"products": &graphql.Field{
			Type: graphql.NewList(ProductType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var (
					productIDs []uint
					order      = p.Source.(Order)
					keys       dataloader.Keys
					v          = p.Context.Value
					loaders    = v("loaders").(map[string]*dataloader.Loader)
					c          = v("client").(*Client)

					handleErrors = func(errors []error) error {
						var errs []string
						for _, e := range errors {
							errs = append(errs, e.Error())
						}
						return fmt.Errorf(strings.Join(errs, "\n"))
					}
				)

				for _, orderItem := range order.OrderItems {
					productIDs = append(productIDs, orderItem.ProductID)
				}

				for _, productID := range productIDs {
					key := NewResolverKey(fmt.Sprintf("%d", productID), c)
					keys = append(keys, key)
				}

				thunk := loaders["GetProducts"].LoadMany(p.Context, keys)
				return func() (interface{}, error) {
					products, errs := thunk()
					if len(errs) > 0 {
						return nil, handleErrors(errs)
					}
					return products, nil
				}, nil
			},
		},
	},
})

var UserType = graphql.NewObject(graphql.ObjectConfig{
	Name: "User",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.Int},
		"firstName": &graphql.Field{Type: graphql.String},
		"lastName":  &graphql.Field{Type: graphql.String},
		"orders": &graphql.Field{
			Type: graphql.NewList(OrderType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var (
					v       = p.Context.Value
					c       = v("client").(*Client)
					loaders = v("loaders").(map[string]*dataloader.Loader)
					user    = p.Source.(*User)
					key     = NewResolverKey(fmt.Sprintf("%d", user.ID), c)
				)
				thunk := loaders["GetUserOrders"].Load(p.Context, key)
				return func() (interface{}, error) {
					return thunk()
				}, nil
			},
		},
	},
})

var QueryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"currentUser": &graphql.Field{
			Type: UserType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var user = p.Context.Value("currentUser").(*User)
				return user, nil
			},
		},
		"categories": &graphql.Field{
			Type: graphql.NewList(CategoryType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var (
					// Same categories from `Client.ListCategories`
					// with ID (1, 2, 3).
					// `GetCategoryBatchFn` batch them.
					categoryIDs []uint = []uint{1, 2, 3}
					keys        dataloader.Keys
					v           = p.Context.Value
					loaders     = v("loaders").(map[string]*dataloader.Loader)
					c           = v("client").(*Client)

					handleErrors = func(errors []error) error {
						var errs []string
						for _, e := range errors {
							errs = append(errs, e.Error())
						}
						return fmt.Errorf(strings.Join(errs, "\n"))
					}
				)

				for _, categoryID := range categoryIDs {
					key := NewResolverKey(fmt.Sprintf("%d",
						categoryID), c)
					keys = append(keys, key)
				}

				thunk := loaders["GetCategory"].LoadMany(p.Context, keys)
				return func() (interface{}, error) {
					categories, errs := thunk()
					if len(errs) > 0 {
						return nil, handleErrors(errs)
					}
					return categories, nil
				}, nil
			},
		},
	},
})

func main() {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: QueryType,
	})
	if err != nil {
		log.Fatal(err)
	}
	var loaders = make(map[string]*dataloader.Loader, 1)
	var client = Client{}
	loaders["GetUserOrders"] = dataloader.NewBatchedLoader(GetUserOrdersBatchFn)
	loaders["GetCategory"] = dataloader.NewBatchedLoader(GetCategoryBatchFn)
	loaders["GetProducts"] = dataloader.NewBatchedLoader(GetProductsBatchFn)
	query := `
		query {
			currentUser {
				id
				firstName
				lastName
				orders {
						id
						products {
								id
								title
								categories {
										id
										name
								}
						}
				}
			}
			categories {
				id
				name
			}
		}
	`
	currentUser := User{
		ID:        1,
		FirstName: "user#1 first name",
		LastName:  "user#1 last name",
	}
	ctx := context.WithValue(context.Background(), "currentUser", &currentUser)
	ctx = context.WithValue(ctx, "loaders", loaders)
	ctx = context.WithValue(ctx, "client", &client)
	result := graphql.Do(graphql.Params{
		Context:       ctx,
		RequestString: query,
		Schema:        schema,
	})
	b, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[GraphQL] result: %s", b)
}

func GetProductsBatchFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}
	var productIDs []uint
	for _, key := range keys {
		id, err := strconv.ParseUint(key.String(), 10, 32)
		if err != nil {
			return handleError(err)
		}
		productIDs = append(productIDs, uint(id))
	}
	products, err := keys[0].(*ResolverKey).client().ListProducts(productIDs)
	if err != nil {
		return handleError(err)
	}

	var productsMap = make(map[uint]Product, len(productIDs))
	for _, product := range products {
		productsMap[product.ID] = product
	}

	var results []*dataloader.Result
	for _, productID := range productIDs {
		product, ok := productsMap[productID]
		if !ok {
			err := errors.New(fmt.Sprintf("product not found, "+
				"product_id: %d", productID))
			return handleError(err)
		}
		result := dataloader.Result{
			Data:  product,
			Error: nil,
		}
		results = append(results, &result)
	}
	log.Printf("[GetProductsBatchFn] batch size: %d", len(results))
	return results
}

func GetUserOrdersBatchFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}
	var userIDs []uint
	for _, key := range keys {
		id, err := strconv.ParseUint(key.String(), 10, 32)
		if err != nil {
			return handleError(err)
		}
		userIDs = append(userIDs, uint(id))
	}
	orders, err := keys[0].(*ResolverKey).client().ListUserOrders(userIDs)
	if err != nil {
		return handleError(err)
	}

	var usersMap = make(map[uint][]Order, len(userIDs))
	for _, order := range orders {
		if _, found := usersMap[order.UserID]; found {
			usersMap[order.UserID] = append(
				usersMap[order.UserID], order)
		} else {
			usersMap[order.UserID] = []Order{order}
		}
	}

	var results []*dataloader.Result
	for _, userID := range userIDs {
		orders, ok := usersMap[userID]
		if !ok {
			err := errors.New(fmt.Sprintf("orders not found, "+
				"user id: %d", userID))
			return handleError(err)
		}
		result := dataloader.Result{
			Data:  orders,
			Error: nil,
		}
		results = append(results, &result)
	}
	log.Printf("[GetUserOrdersBatchFn] batch size: %d", len(results))
	return results
}

func GetCategoryBatchFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
	var results []*dataloader.Result
	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}
	var categoryIDs []uint
	for _, key := range keys {
		id, err := strconv.ParseUint(key.String(), 10, 32)
		if err != nil {
			return handleError(err)
		}
		categoryIDs = append(categoryIDs, uint(id))
	}
	categories, err := keys[0].(*ResolverKey).client().ListCategories(categoryIDs)
	if err != nil {
		return handleError(err)
	}
	var categoryMap = make(map[uint]Category, len(categoryIDs))
	for _, category := range categories {
		categoryMap[category.ID] = category
	}
	for _, category := range categories {
		category = categoryMap[category.ID]
		result := dataloader.Result{
			Data:  category,
			Error: nil,
		}
		results = append(results, &result)
	}
	log.Printf("[GetCategoryBatchFn] batch size: %d", len(results))
	return results
}
