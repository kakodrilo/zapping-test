DELIMITER //

CREATE PROCEDURE sp_get_user_by_email(
    IN p_email VARCHAR(150)
)
BEGIN
    SELECT 
        id, 
        name, 
        email, 
        password_hash 
    FROM users 
    WHERE email = p_email 
    LIMIT 1;
END //

DELIMITER ;