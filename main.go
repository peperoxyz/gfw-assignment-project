package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var (
	db  *sql.DB
	err error
)

// const (
// 	user     = "root"
// 	password = ""
// 	dbName   = "order_assignment"
// )

func dbConn() (*sql.DB, error) {
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_ROOT_PASSWORD"), os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_PORT"), os.Getenv("MYSQL_DATABASE"))
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, fmt.Errorf("error opening database: %v", err)
    }

    err = db.Ping()
    if err != nil {
        return nil, fmt.Errorf("error pinging database: %v", err)
    }

    fmt.Println("Successfully connected to the database")
    return db, nil
}

// Order struct
type Order struct {
	ID          	int    `json:"id"`
	CustomerName 	string `json:"customerName"`
	OrderedAt		string `json:"orderedAt"`
	UpdatedAt		string `json:"updatedAt,omitempty"`
	Items			[]Item `json:"items"` // Add Items slice to Order struct
}

// Item struct
type Item struct {
	ID          	int    `json:"id,omitempty"`
	Name 			string `json:"name"`
	Description		string `json:"description"`
	Quantity		int    `json:"quantity"`
	OrderID			int    `json:"order_id,omitempty"`
	CreatedAt		string `json:"created_at,omitempty"`
	UpdatedAt		string `json:"updated_at,omitempty"`
}

func main() {
	db, err = dbConn()
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}
	defer db.Close()

	// Initialize Gin router
	router := gin.Default()

	// Define the route
	router.POST("/orders", createOrder)
	router.GET("/orders", getOrders)
	router.GET("/orders/:id", getOrder)
	router.PUT("/orders/:id", updateOrder)
	router.DELETE("/orders/:id", deleteOrder)

	// Run the server
	router.Run(":9090")
}

// Create Order
func createOrder(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the incoming order data
	log.Printf("Received order: %+v", order)

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Insert order
	result, err := tx.Exec("INSERT INTO orders (customer_name, ordered_at) VALUES (?, ?)", order.CustomerName, order.OrderedAt)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log the result of the order insert
	log.Printf("Order insert result: %+v", result)

	// Get the order_id of the newly inserted order
	orderID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the new order ID
	log.Printf("New order ID: %d", orderID)

	// Insert items
	for _, item := range order.Items {
		_, err = tx.Exec("INSERT INTO items (name, description, quantity, order_id) VALUES (?, ?, ?, ?)",
			item.Name, item.Description, item.Quantity, orderID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order created successfully"})
}

// Get Orders
func getOrders(c *gin.Context) {
	var orders []Order
	rows, err := db.Query("SELECT id, customer_name, ordered_at FROM orders")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var order Order
		if err := rows.Scan(&order.ID, &order.CustomerName, &order.OrderedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Get items for each order
		rowsItems, err := db.Query("SELECT id, name, description, quantity FROM items WHERE order_id = ?", order.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rowsItems.Close()

		for rowsItems.Next() {
			var item Item
			if err := rowsItems.Scan(&item.ID, &item.Name, &item.Description, &item.Quantity); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			order.Items = append(order.Items, item)
		}

		orders = append(orders, order)
	}

	c.JSON(http.StatusOK, orders)
}

// Get Order
func getOrder(c *gin.Context) {
	var order Order
	id := c.Param("id")
	row := db.QueryRow("SELECT id, customer_name, ordered_at FROM orders WHERE id = ?", id)
	if err := row.Scan(&order.ID, &order.CustomerName, &order.OrderedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get items for the order
	rowsItems, err := db.Query("SELECT id, name, description, quantity FROM items WHERE order_id = ?", order.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rowsItems.Close()

	for rowsItems.Next() {
		var item Item
		if err := rowsItems.Scan(&item.ID, &item.Name, &item.Description, &item.Quantity); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		order.Items = append(order.Items, item)
	}

	c.JSON(http.StatusOK, order)
}

// Update Order
func updateOrder(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update order
	_, err = tx.Exec("UPDATE orders SET customer_name = ?, ordered_at = ? WHERE id = ?", order.CustomerName, order.OrderedAt, id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete existing items
	_, err = tx.Exec("DELETE FROM items WHERE order_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Insert items
	for _, item := range order.Items {
		_, err = tx.Exec("INSERT INTO items (name, description, quantity, order_id) VALUES (?, ?, ?, ?)",
			item.Name, item.Description, item.Quantity, id)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order updated successfully"})
}

// Delete Order
func deleteOrder(c *gin.Context) {
	id := c.Param("id")

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete items first to avoid foreign key constraint error
	_, err = tx.Exec("DELETE FROM items WHERE order_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete order
	_, err = tx.Exec("DELETE FROM orders WHERE id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}

