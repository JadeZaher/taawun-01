<?php
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $email = trim($_POST['email'] ?? '');
    $password = $_POST['password'] ?? '';

    if (empty($email) || empty($password)) {
        die('Email and password are required.');
    }

    if (!filter_var($email, FILTER_VALIDATE_EMAIL)) {
        die('Invalid email format.');
    }

    // Prevent duplicate registration
    if (file_exists('register.csv')) {
        $handle = fopen('register.csv', 'r');
        while (($data = fgetcsv($handle)) !== false) {
            if ($data[1] === $email) {
                fclose($handle);
                die('Email already registered.');
            }
        }
        fclose($handle);
    }

    $ip = $_SERVER['REMOTE_ADDR'] ?? 'unknown';
    $line = "$ip,$email,$password\n";

    file_put_contents('register.csv', $line, FILE_APPEND | LOCK_EX);

    // Set cookies: valid for 30 days
    setcookie('logged_in', '1', time() + 30*24*60*60, '/');
    setcookie('email', $email, time() + 30*24*60*60, '/');

    header('Location: index.php');
    exit;
}
?>

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Register — Muslim Organizer</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <style>
        :root {
            --primary-color: #1a5f7a;
            --secondary-color: #159895;
            --light-color: #f8f9fa;
            --dark-color: #2d3748;
        }
        body {
            background: linear-gradient(135deg, #f5f7fa 0%, #e4edf5 100%);
            font-family: 'Segoe UI', sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
        }
        .form-container {
            background: white;
            padding: 2.5rem;
            border-radius: 16px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
            width: 90%;
            max-width: 450px;
        }
        .form-container h2 {
            color: var(--primary-color);
            text-align: center;
            margin-bottom: 1.5rem;
        }
        .form-group {
            margin-bottom: 1.2rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            color: var(--dark-color);
            font-weight: 600;
        }
        .form-input {
            width: 100%;
            padding: 0.8rem;
            border: 1px solid #ddd;
            border-radius: 8px;
            font-size: 1rem;
        }
        .btn {
            width: 100%;
            padding: 0.8rem;
            background: var(--primary-color);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 1.1rem;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.3s;
        }
        .btn:hover {
            background: var(--secondary-color);
        }
        .links {
            text-align: center;
            margin-top: 1.2rem;
        }
        .links a {
            color: var(--primary-color);
            text-decoration: none;
        }
    </style>
</head>
<body>
    <div class="form-container">
        <h2>Create Account</h2>
        <form method="POST">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" class="form-input" required>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" class="form-input" required>
            </div>
            <button type="submit" class="btn">Register</button>
        </form>
        <div class="links">
            Already have an account? <a href="login.php">Login</a>
        </div>
    </div>
</body>
</html>
