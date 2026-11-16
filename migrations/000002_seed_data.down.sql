DELETE FROM inventory_transactions WHERE reference LIKE 'Initial warehouse delivery%' OR reference LIKE 'Exclusive launch%';
DELETE FROM inventory WHERE product_id IN (1, 2, 3, 4, 5);
DELETE FROM products WHERE id IN (1, 2, 3, 4, 5);
DELETE FROM categories WHERE id IN (1, 2, 3);
