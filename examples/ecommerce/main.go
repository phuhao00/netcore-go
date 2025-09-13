// E-commerce Backend Example - NetCore-Go
// Microservices architecture for e-commerce platform
// Author: NetCore-Go Team
// Created: 2024

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Product represents a product in the catalog
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Stock       int     `json:"stock"`
	ImageURL    string  `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Order represents a customer order
type Order struct {
	ID         string      `json:"id"`
	CustomerID string      `json:"customer_id"`
	Items      []OrderItem `json:"items"`
	Total      float64     `json:"total"`
	Status     string      `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// Customer represents a customer
type Customer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

// Cart represents a shopping cart
type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Items      []CartItem `json:"items"`
	Total      float64    `json:"total"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CartItem represents an item in the cart
type CartItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// ProductService handles product operations
type ProductService struct {
	mu       sync.RWMutex
	products map[string]*Product
}

// NewProductService creates a new product service
func NewProductService() *ProductService {
	service := &ProductService{
		products: make(map[string]*Product),
	}
	service.addSampleProducts()
	return service
}

func (ps *ProductService) addSampleProducts() {
	products := []*Product{
		{
			ID:          uuid.New().String(),
			Name:        "Wireless Headphones",
			Description: "High-quality wireless headphones with noise cancellation",
			Price:       199.99,
			Category:    "Electronics",
			Stock:       50,
			ImageURL:    "/images/headphones.jpg",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Name:        "Smart Watch",
			Description: "Fitness tracking smartwatch with heart rate monitor",
			Price:       299.99,
			Category:    "Electronics",
			Stock:       30,
			ImageURL:    "/images/smartwatch.jpg",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          uuid.New().String(),
			Name:        "Running Shoes",
			Description: "Comfortable running shoes for daily exercise",
			Price:       89.99,
			Category:    "Sports",
			Stock:       100,
			ImageURL:    "/images/shoes.jpg",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	for _, product := range products {
		ps.products[product.ID] = product
	}
}

func (ps *ProductService) GetAllProducts() []*Product {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	
	var products []*Product
	for _, product := range ps.products {
		products = append(products, product)
	}
	return products
}

func (ps *ProductService) GetProductByID(id string) (*Product, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	
	product, exists := ps.products[id]
	return product, exists
}

func (ps *ProductService) CreateProduct(product *Product) *Product {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	product.ID = uuid.New().String()
	product.CreatedAt = time.Now()
	product.UpdatedAt = time.Now()
	ps.products[product.ID] = product
	return product
}

func (ps *ProductService) UpdateStock(productID string, quantity int) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	
	product, exists := ps.products[productID]
	if !exists {
		return fmt.Errorf("product not found")
	}
	
	if product.Stock < quantity {
		return fmt.Errorf("insufficient stock")
	}
	
	product.Stock -= quantity
	product.UpdatedAt = time.Now()
	return nil
}

// OrderService handles order operations
type OrderService struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

func NewOrderService() *OrderService {
	return &OrderService{
		orders: make(map[string]*Order),
	}
}

func (os *OrderService) CreateOrder(order *Order) *Order {
	os.mu.Lock()
	defer os.mu.Unlock()
	
	order.ID = uuid.New().String()
	order.Status = "pending"
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	os.orders[order.ID] = order
	return order
}

func (os *OrderService) GetOrderByID(id string) (*Order, bool) {
	os.mu.RLock()
	defer os.mu.RUnlock()
	
	order, exists := os.orders[id]
	return order, exists
}

func (os *OrderService) GetOrdersByCustomer(customerID string) []*Order {
	os.mu.RLock()
	defer os.mu.RUnlock()
	
	var orders []*Order
	for _, order := range os.orders {
		if order.CustomerID == customerID {
			orders = append(orders, order)
		}
	}
	return orders
}

func (os *OrderService) UpdateOrderStatus(id, status string) error {
	os.mu.Lock()
	defer os.mu.Unlock()
	
	order, exists := os.orders[id]
	if !exists {
		return fmt.Errorf("order not found")
	}
	
	order.Status = status
	order.UpdatedAt = time.Now()
	return nil
}

// CartService handles shopping cart operations
type CartService struct {
	mu    sync.RWMutex
	carts map[string]*Cart
}

func NewCartService() *CartService {
	return &CartService{
		carts: make(map[string]*Cart),
	}
}

func (cs *CartService) GetCart(customerID string) *Cart {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cart, exists := cs.carts[customerID]
	if !exists {
		cart = &Cart{
			ID:         uuid.New().String(),
			CustomerID: customerID,
			Items:      []CartItem{},
			Total:      0,
			UpdatedAt:  time.Now(),
		}
		cs.carts[customerID] = cart
	}
	return cart
}

func (cs *CartService) AddToCart(customerID, productID string, quantity int, price float64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cart := cs.GetCart(customerID)
	
	// Check if item already exists
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items[i].Quantity += quantity
			cs.updateCartTotal(cart)
			return
		}
	}
	
	// Add new item
	cart.Items = append(cart.Items, CartItem{
		ProductID: productID,
		Quantity:  quantity,
		Price:     price,
	})
	cs.updateCartTotal(cart)
}

func (cs *CartService) RemoveFromCart(customerID, productID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cart := cs.GetCart(customerID)
	
	for i, item := range cart.Items {
		if item.ProductID == productID {
			cart.Items = append(cart.Items[:i], cart.Items[i+1:]...)
			break
		}
	}
	cs.updateCartTotal(cart)
}

func (cs *CartService) ClearCart(customerID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cart := cs.GetCart(customerID)
	cart.Items = []CartItem{}
	cart.Total = 0
	cart.UpdatedAt = time.Now()
}

func (cs *CartService) updateCartTotal(cart *Cart) {
	total := 0.0
	for _, item := range cart.Items {
		total += item.Price * float64(item.Quantity)
	}
	cart.Total = total
	cart.UpdatedAt = time.Now()
}

// HTTP Handlers
type EcommerceHandler struct {
	productService *ProductService
	orderService   *OrderService
	cartService    *CartService
}

func NewEcommerceHandler(ps *ProductService, os *OrderService, cs *CartService) *EcommerceHandler {
	return &EcommerceHandler{
		productService: ps,
		orderService:   os,
		cartService:    cs,
	}
}

func (eh *EcommerceHandler) handleProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodGet:
		products := eh.productService.GetAllProducts()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"products": products,
			"total":    len(products),
		})
		
	case http.MethodPost:
		var product Product
		if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		createdProduct := eh.productService.CreateProduct(&product)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createdProduct)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (eh *EcommerceHandler) handleProductByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Extract ID from URL path
	path := r.URL.Path
	id := path[len("/api/products/"):]
	
	product, exists := eh.productService.GetProductByID(id)
	if !exists {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(product)
}

func (eh *EcommerceHandler) handleCart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		http.Error(w, "Customer ID required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		cart := eh.cartService.GetCart(customerID)
		json.NewEncoder(w).Encode(cart)
		
	case http.MethodPost:
		var req struct {
			ProductID string  `json:"product_id"`
			Quantity  int     `json:"quantity"`
			Price     float64 `json:"price"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		eh.cartService.AddToCart(customerID, req.ProductID, req.Quantity, req.Price)
		cart := eh.cartService.GetCart(customerID)
		json.NewEncoder(w).Encode(cart)
		
	case http.MethodDelete:
		productID := r.URL.Query().Get("product_id")
		if productID != "" {
			eh.cartService.RemoveFromCart(customerID, productID)
		} else {
			eh.cartService.ClearCart(customerID)
		}
		
		cart := eh.cartService.GetCart(customerID)
		json.NewEncoder(w).Encode(cart)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (eh *EcommerceHandler) handleOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	switch r.Method {
	case http.MethodPost:
		var order Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		
		// Validate stock and update inventory
		for _, item := range order.Items {
			if err := eh.productService.UpdateStock(item.ProductID, item.Quantity); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		
		createdOrder := eh.orderService.CreateOrder(&order)
		
		// Clear cart after successful order
		eh.cartService.ClearCart(order.CustomerID)
		
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(createdOrder)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (eh *EcommerceHandler) handleOrderByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Extract ID from URL path
	path := r.URL.Path
	id := path[len("/api/orders/"):]
	
	order, exists := eh.orderService.GetOrderByID(id)
	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(order)
}

func (eh *EcommerceHandler) handleCustomerOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		http.Error(w, "Customer ID required", http.StatusBadRequest)
		return
	}
	
	orders := eh.orderService.GetOrdersByCustomer(customerID)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"total":  len(orders),
	})
}

func main() {
	// Initialize services
	productService := NewProductService()
	orderService := NewOrderService()
	cartService := NewCartService()
	
	// Initialize handler
	handler := NewEcommerceHandler(productService, orderService, cartService)
	
	// Setup routes
	http.HandleFunc("/api/products", handler.handleProducts)
	http.HandleFunc("/api/products/", handler.handleProductByID)
	http.HandleFunc("/api/cart", handler.handleCart)
	http.HandleFunc("/api/orders", handler.handleOrders)
	http.HandleFunc("/api/orders/", handler.handleOrderByID)
	http.HandleFunc("/api/customer/orders", handler.handleCustomerOrders)
	
	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"service":   "ecommerce-backend",
			"version":   "1.0.0",
		})
	})
	
	// Root endpoint with API documentation
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "NetCore-Go E-commerce Backend",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"GET /api/products":           "Get all products",
				"POST /api/products":          "Create a new product",
				"GET /api/products/{id}":      "Get product by ID",
				"GET /api/cart?customer_id=X": "Get customer cart",
				"POST /api/cart?customer_id=X": "Add item to cart",
				"DELETE /api/cart?customer_id=X": "Remove from cart",
				"POST /api/orders":            "Create order",
				"GET /api/orders/{id}":        "Get order by ID",
				"GET /api/customer/orders?customer_id=X": "Get customer orders",
				"GET /health":                 "Health check",
			},
			"microservices": []string{
				"Product Catalog Service",
				"Order Management Service",
				"Shopping Cart Service",
				"Inventory Management",
			},
		})
	})
	
	port := ":8081"
	fmt.Printf("🛒 E-commerce Backend starting on port %s\n", port)
	fmt.Println("📖 API Documentation: http://localhost:8081")
	fmt.Println("🏥 Health Check: http://localhost:8081/health")
	fmt.Println("🛍️ Products: http://localhost:8081/api/products")
	
	log.Fatal(http.ListenAndServe(port, nil))
}