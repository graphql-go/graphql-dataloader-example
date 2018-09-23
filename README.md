# graphql-dataloader-example

An example project on how to use [github.com/graphql-go/graphql](https://github.com/graphql-go/graphql) with [github.com/graph-gophers/dataloader](https://github.com/graph-gophers/dataloader) to support batching to avoid n+1 queries.

#### Getting Started

To run the example:
```bash
go run main.go
```

Output:
```
2018/09/22 16:03:55 [GetCategoryBatchFn] batch size: 3
2018/09/22 16:03:55 [GetUserOrdersBatchFn] batch size: 1
2018/09/22 16:03:55 [GetProductsBatchFn] batch size: 1
2018/09/22 16:03:55 [GraphQL] result: {"data":{"categories":[{"id":1,"name":"name#1"},{"id":2,"name":"name#2"},{"id":3,"name":"name#3"}],"currentUser":{"firstName":"user#1 first name","id":1,"lastName":"user#1 last name","orders":[{"id":200,"products":[{"categories":[{"id":3,"name":"name#3"},{"id":1,"name":"name#1"},{"id":2,"name":"name#2"}],"id":100,"title":"product#100"}]}]}}}
```

GraphQL result pretty print using [jq](https://github.com/stedolan/jq):
```bash

(graphql-dataloader-example)-> echo '{"data":{"categories":[{"id":1,"name":"name#1"},{"id":2,"name":"name#2"},{"id":3,"name":"name#3"}],"currentUser":{"firstName":"user#1 first name","id":1,"lastName":"user#1 last name","orders":[{"id":200,"products":[{"categories":[{"id":3,"name":"name#3"},{"id":1,"name":"name#1"},{"id":2,"name":"name#2"}],"id":100,"title":"product#100"}]}]}}}' | jq
```

```json
{
  "data": {
    "categories": [
      {
        "id": 1,
        "name": "name#1"
      },
      {
        "id": 2,
        "name": "name#2"
      },
      {
        "id": 3,
        "name": "name#3"
      }
    ],
    "currentUser": {
      "firstName": "user#1 first name",
      "id": 1,
      "lastName": "user#1 last name",
      "orders": [
        {
          "id": 200,
          "products": [
            {
              "categories": [
                {
                  "id": 3,
                  "name": "name#3"
                },
                {
                  "id": 1,
                  "name": "name#1"
                },
                {
                  "id": 2,
                  "name": "name#2"
                }
              ],
              "id": 100,
              "title": "product#100"
            }
          ]
        }
      ]
    }
  }
}
```
