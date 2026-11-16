-- Seed Categories
INSERT INTO categories (id, name, description) VALUES
(1, 'Electronics', 'Smartphones, laptops, monitors and consumer electronic goods'),
(2, 'Home & Office', 'Ergonomic chairs, desks, lighting and work essentials'),
(3, 'Apparel', 'Sportswear, jackets, footwear and accessories')
ON CONFLICT (id) DO NOTHING;

-- Seed Products
INSERT INTO products (id, name, description, price, category_id, sku, image_url, is_active) VALUES
(1, 'MacBook Pro 16 M3 Max', 'Apple M3 Max 16-core CPU, 40-core GPU, 64GB Unified Memory, 1TB SSD', 3499.00, 1, 'SKU-MBP16-M3MAX', 'https://images.unsplash.com/photo-1517336714731-489689fd1ca8', true),
(2, 'Sony WH-1000XM5 Wireless Headphones', 'Industry-leading noise canceling with Auto NC Optimizer and 30-hour battery life', 398.00, 1, 'SKU-SONY-XM5', 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e', true),
(3, 'Herman Miller Aeron Ergonomic Chair', 'PostureFit SL, adjustable arms, breathable Pellicle suspension', 1295.00, 2, 'SKU-HM-AERON-B', 'https://images.unsplash.com/photo-1580481077195-c22ae2b33058', true),
(4, 'Keychron Q1 Pro Wireless Mechanical Keyboard', 'QMK/VIA wireless custom mechanical keyboard with CNC aluminum body', 199.00, 2, 'SKU-KC-Q1PRO', 'https://images.unsplash.com/photo-1587829741301-dc798b83add3', true),
(5, 'Limited Edition Cyberpunk Jacket', 'Waterproof cyber-aesthetic streetwear jacket with LED trim - HIGH DEMAND (1 LEFT)', 450.00, 3, 'SKU-CP-JACKET-LTD', 'https://images.unsplash.com/photo-1551028719-00167b16eac5', true)
ON CONFLICT (id) DO NOTHING;

-- Reset sequence for products and categories
SELECT setval('products_id_seq', (SELECT MAX(id) FROM products));
SELECT setval('categories_id_seq', (SELECT MAX(id) FROM categories));

-- Seed Inventory
-- Notice: Product 5 has quantity = 1, perfect for demonstrating concurrent race condition tests!
INSERT INTO inventory (product_id, quantity, reserved_quantity, min_stock_level, last_restocked_at) VALUES
(1, 50, 0, 10, NOW()),
(2, 120, 0, 20, NOW()),
(3, 15, 0, 5, NOW()),
(4, 75, 0, 15, NOW()),
(5, 1, 0, 1, NOW())
ON CONFLICT (product_id) DO UPDATE 
SET quantity = EXCLUDED.quantity, reserved_quantity = 0;

-- Initial Restock Transaction Audit Records
INSERT INTO inventory_transactions (product_id, type, quantity, reference) VALUES
(1, 'restock', 50, 'Initial warehouse delivery #WH-001'),
(2, 'restock', 120, 'Initial warehouse delivery #WH-001'),
(3, 'restock', 15, 'Initial warehouse delivery #WH-001'),
(4, 'restock', 75, 'Initial warehouse delivery #WH-001'),
(5, 'restock', 1, 'Exclusive launch inventory batch #LTD-001');
