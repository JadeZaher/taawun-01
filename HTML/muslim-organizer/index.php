<?php
session_start();

header("Cache-Control: no-store, no-cache, must-revalidate, max-age=0");
header("Pragma: no-cache");
header("Expires: 0");

// Check if user is logged in via cookie
$logged_in = false;
$email = '';
if (isset($_COOKIE['logged_in']) && $_COOKIE['logged_in'] === '1' && isset($_COOKIE['email'])) {
    $logged_in = true;
    $email = htmlspecialchars($_COOKIE['email']);
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title> Title:  Muslim Organizer — Digital Prayer, Learning & Halal Trading</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <style>
        /* Global Styles */
        :root {
            --primary-color: #1a5f7a;
            --secondary-color: #159895;
            --accent-color: #57cc99;
            --light-color: #f8f9fa;
            --dark-color: #2d3748;
            --gray-color: #6c757d;
            --success-color: #28a745;
            --warning-color: #ffc107;
            --danger-color: #dc3545;
            --border-radius: 12px;
            --box-shadow: 0 6px 15px rgba(0, 0, 0, 0.08);
            --transition: all 0.3s ease;
            /* Gen Z Colors */
            --tech: #6A4CFF;
            --morality: #00C897;
            --freelance: #FF6B6B;
            --creative: #FF9A3D;
            --trading: #4D96FF;
            /* Trading Colors */
            --trading-dark: #1a2a6c;
            --trading-red: #b21f1f;
            --trading-yellow: #fdbb2d;
            --trading-gold: #ffcc00;
            /* Backgrounds */
            --background-organizer: linear-gradient(135deg, #f5f7fa 0%, #e4edf5 100%);
            --background-gen-z: #121212;
            --background-trading: linear-gradient(135deg, var(--trading-dark), var(--trading-red), var(--trading-yellow));
        }
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
        }
        body {
            background: var(--background-organizer);
            color: var(--dark-color);
            line-height: 1.6;
            min-height: 100vh;
        }
        .app-container {
            display: flex;
            flex-direction: column;
            min-height: 100vh;
        }
        /* Header Styles */
        .app-header {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            color: white;
            padding: 1rem 2rem;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
            display: flex;
            justify-content: space-between;
            align-items: center;
            position: sticky;
            top: 0;
            z-index: 100;
        }
        .logo {
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .logo-icon {
            font-size: 1.8rem;
            background: rgba(255, 255, 255, 0.2);
            width: 50px;
            height: 50px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .logo h1 {
            font-size: 1.5rem;
            font-weight: 700;
        }
        .main-nav {
            display: flex;
            gap: 1.5rem;
        }
        .nav-item {
            text-decoration: none;
            color: rgba(255, 255, 255, 0.9);
            font-weight: 500;
            padding: 0.6rem 1.2rem;
            border-radius: 30px;
            transition: var(--transition);
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .nav-item i {
            font-size: 1.1rem;
        }
        .nav-item.active,
        .nav-item:hover {
            background-color: rgba(255, 255, 255, 0.15);
            color: white;
            transform: translateY(-2px);
        }
        .user-actions {
            display: flex;
            gap: 1rem;
        }
        .btn {
            padding: 0.6rem 1.5rem;
            border: none;
            border-radius: 30px;
            cursor: pointer;
            font-weight: 600;
            transition: var(--transition);
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .btn-primary {
            background: white;
            color: var(--primary-color);
        }
        .btn-primary:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        }
        .btn-outline {
            background: transparent;
            border: 2px solid rgba(255, 255, 255, 0.7);
            color: white;
        }
        .btn-outline:hover {
            background: rgba(255, 255, 255, 0.1);
            transform: translateY(-2px);
        }
        /* Main Content */
        .main-content {
            flex: 1;
            padding: 2rem;
            max-width: 1400px;
            margin: 0 auto;
            width: 100%;
        }
        body.quran-study-active .main-content {
            padding: 0 !important;
            max-width: none;
            margin: 0;
        }
        .section {
            display: none;
            padding: 2rem 0;
        }
        .section.active {
            display: block;
        }
        .section-title {
            font-size: 2.2rem;
            color: var(--primary-color);
            margin-bottom: 2rem;
            font-weight: 700;
            text-align: center;
        }
        /* Home Section */
        .hero-section {
            text-align: center;
            padding: 0rem 1rem;
            margin-bottom: 2rem;
        }
        .hero-section h2 {
            font-size: 2.5rem;
            color: var(--primary-color);
            margin-bottom: 1rem;
            font-weight: 700;
        }
        .hero-section p {
            font-size: 1.2rem;
            color: var(--gray-color);
            max-width: 700px;
            margin: 0 auto 2rem;
        }
        .cta-buttons {
            display: flex;
            gap: 1rem;
            justify-content: center;
            margin-top: 2rem;
            flex-wrap: wrap;
        }
        .cta-btn {
            padding: 0.8rem 2rem;
            border-radius: 30px;
            font-weight: 600;
            font-size: 1.1rem;
            display: flex;
            align-items: center;
            gap: 10px;
            transition: var(--transition);
        }
        .cta-primary {
            background: var(--primary-color);
            color: white;
            border: none;
        }
        .cta-secondary {
            background: white;
            color: var(--primary-color);
            border: 2px solid var(--primary-color);
        }
        .cta-primary:hover, .cta-secondary:hover {
            transform: translateY(-3px);
            box-shadow: 0 6px 15px rgba(0, 0, 0, 0.1);
        }
        /* Dashboard Grid */
        .dashboard-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 1.5rem;
            margin-top: 2rem;
        }
        .dashboard-card {
            background: white;
            border-radius: var(--border-radius);
            box-shadow: var(--box-shadow);
            padding: 1.5rem;
            transition: var(--transition);
            border-top: 4px solid var(--primary-color);
        }
        .dashboard-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 20px rgba(0, 0, 0, 0.12);
        }
        .card-header {
            display: flex;
            align-items: center;
            margin-bottom: 1rem;
        }
        .card-icon {
            width: 50px;
            height: 50px;
            border-radius: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            margin-right: 1rem;
            font-size: 1.5rem;
            color: white;
        }
        .icon-quran {
            background: linear-gradient(135deg, #8e44ad, #9b59b6);
        }
        .icon-prayer {
            background: linear-gradient(135deg, #3498db, #2980b9);
        }
        .icon-organizer {
            background: linear-gradient(135deg, var(--secondary-color), var(--primary-color));
        }
        .icon-audio {
            background: linear-gradient(135deg, #e74c3c, #c0392b);
        }
        .icon-notes {
            background: linear-gradient(135deg, #f39c12, #e67e22);
        }
        .icon-resources {
            background: linear-gradient(135deg, #27ae60, #2ecc71);
        }
        .card-title {
            font-size: 1.3rem;
            font-weight: 600;
            color: var(--dark-color);
        }
        .card-content {
            color: var(--gray-color);
            margin-bottom: 1.5rem;
            line-height: 1.5;
        }
        .card-stats {
            display: flex;
            justify-content: space-between;
            border-top: 1px solid #eee;
            padding-top: 1rem;
        }
        .stat {
            text-align: center;
        }
        .stat-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary-color);
        }
        .stat-label {
            font-size: 0.8rem;
            color: var(--gray-color);
            text-transform: uppercase;
        }
        /* Prayer Times Widget */
        .prayer-widget {
            background: linear-gradient(135deg, var(--primary-color), var(--secondary-color));
            border-radius: var(--border-radius);
            padding: 2rem;
            color: white;
            margin: 3rem 0;
            box-shadow: var(--box-shadow);
        }
        .widget-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
        }
        .widget-title {
            font-size: 1.5rem;
            font-weight: 600;
        }
        .prayer-times {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 1rem;
        }
        .prayer-time {
            text-align: center;
            padding: 1rem;
            background: rgba(255, 255, 255, 0.1);
            border-radius: var(--border-radius);
            transition: var(--transition);
        }
        .prayer-time.active {
            background: rgba(255, 255, 255, 0.2);
            transform: scale(1.05);
        }
        .prayer-name {
            font-size: 1rem;
            margin-bottom: 0.5rem;
            opacity: 0.9;
        }
        .prayer-hour {
            font-size: 1.5rem;
            font-weight: 700;
        }
        /* Features Section */
        .features-section {
            margin: 4rem 0;
        }
        .features-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 1.5rem;
        }
        .feature-card {
            background: white;
            border-radius: var(--border-radius);
            box-shadow: var(--box-shadow);
            padding: 2rem;
            text-align: center;
            transition: var(--transition);
        }
        .feature-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
        }
        .feature-icon {
            width: 70px;
            height: 70px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0 auto 1.5rem;
            font-size: 1.8rem;
            color: white;
            background: linear-gradient(135deg, var(--primary-color), var(--secondary-color));
        }
        .feature-title {
            font-size: 1.3rem;
            font-weight: 600;
            margin-bottom: 1rem;
            color: var(--dark-color);
        }
        .feature-desc {
            color: var(--gray-color);
            line-height: 1.6;
        }
        /* Gen Z Section */
        #gen-z {
            background-color: var(--background-gen-z);
            color: white;
            padding: 2rem 0;
        }
        #gen-z .section-title {
            color: white;
            margin-top: 0;
        }
        .gen-z-container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 2rem;
        }
        .tagline {
            font-size: 1.8rem;
            margin-bottom: 3rem;
            color: #CCCCCC;
            text-align: center;
        }
        .domains {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 2rem;
            margin-bottom: 4rem;
        }
        .domain {
            padding: 2rem;
            border-radius: 16px;
            transition: transform 0.3s ease;
            cursor: pointer;
            position: relative;
            overflow: hidden;
            background: rgba(255, 255, 255, 0.05);
            border: 2px solid;
        }
        .domain:hover {
            transform: translateY(-10px);
        }
        .domain::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            height: 5px;
        }
        .domain-tech {
            border-color: var(--tech);
        }
        .domain-tech::before {
            background: var(--tech);
        }
        .domain-morality {
            border-color: var(--morality);
        }
        .domain-morality::before {
            background: var(--morality);
        }
        .domain-freelance {
            border-color: var(--freelance);
        }
        .domain-freelance::before {
            background: var(--freelance);
        }
        .domain-creative {
            border-color: var(--creative);
        }
        .domain-creative::before {
            background: var(--creative);
        }
        .domain-trading {
            border-color: var(--trading);
        }
        .domain-trading::before {
            background: var(--trading);
        }
        .domain h2 {
            font-size: 2rem;
            margin-bottom: 1rem;
            color: var(--tech);
        }
        .domain p {
            font-size: 1.1rem;
            margin-bottom: 1.5rem;
            color: #CCCCCC;
        }
        .domain ul {
            list-style-type: none;
            font-size: 1rem;
        }
        .domain li {
            margin-bottom: 0.5rem;
            padding-left: 1.5rem;
            position: relative;
        }
        .domain li::before {
            content: '→';
            position: absolute;
            left: 0;
            color: var(--tech);
        }
        .cta-section {
            text-align: center;
            padding: 3rem 2rem;
            background: rgba(255, 255, 255, 0.05);
            border-radius: 20px;
            margin-bottom: 3rem;
            max-width: 900px;
            margin: 3rem auto;
        }
        .cta-section h2 {
            font-size: 2.5rem;
            margin-bottom: 1.5rem;
            color: var(--creative);
        }
        .cta-section p {
            font-size: 1.3rem;
            margin-bottom: 2rem;
            max-width: 800px;
            margin-left: auto;
            margin-right: auto;
        }
        .cta-button {
            display: inline-block;
            padding: 1.2rem 3rem;
            background: linear-gradient(45deg, var(--tech), var(--freelance));
            color: white;
            text-decoration: none;
            font-size: 1.3rem;
            font-weight: bold;
            border-radius: 50px;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .cta-button:hover {
            transform: scale(1.05);
            box-shadow: 0 10px 25px rgba(106, 76, 255, 0.4);
        }
        .research-objectives h2 {
            font-size: 2.2rem;
            margin-bottom: 2rem;
            text-align: center;
            color: var(--morality);
        }
        .objectives-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1.5rem;
        }
        .objective {
            padding: 1.5rem;
            background: rgba(255, 255, 255, 0.05);
            border-radius: 12px;
            text-align: center;
        }
        .objective h3 {
            font-size: 1.3rem;
            margin-bottom: 1rem;
            color: var(--trading);
        }
        .objective p {
            font-size: 1rem;
            color: #CCCCCC;
        }
        .hashtags {
            margin-top: 1.5rem;
            font-size: 1.1rem;
            text-align: center;
            color: #CCCCCC;
        }
        /* Trading Section */
        #trading {
            background: var(--background-trading);
            color: white;
            padding: 2rem 0;
        }
        #trading .section-title {
            color: white;
            margin-top: 0;
        }
        .trading-container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 2rem;
        }
        .subtitle {
            font-size: 1.5rem;
            margin-bottom: 20px;
            color: var(--trading-gold);
            text-align: center;
        }
        .trading-header {
            text-align: center;
            margin-bottom: 30px;
        }
        .trading-header h1 {
            font-size: 2.5rem;
            text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.5);
            margin-bottom: 10px;
        }
        .trading-header p {
            font-size: 1.2rem;
            margin-bottom: 20px;
            max-width: 800px;
            margin: 0 auto;
        }
        .promo-text {
            text-align: center;
            margin-bottom: 30px;
            font-size: 1.1rem;
            line-height: 1.6;
            max-width: 900px;
            margin: 0 auto 30px;
        }
        .promo-text strong {
            color: var(--trading-gold);
        }
        .content {
            display: flex;
            flex-wrap: wrap;
            gap: 30px;
            justify-content: center;
            margin: 30px 0;
        }
        .book-covers {
            flex: 1;
            min-width: 300px;
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .book-cover {
            background: rgba(255, 255, 255, 0.1);
            border-radius: 15px;
            overflow: hidden;
            box-shadow: 0 10px 20px rgba(0, 0, 0, 0.3);
            transition: transform 0.3s ease;
            display: flex;
            flex-direction: column;
            align-items: center;
            padding: 15px;
        }
        .book-cover:hover {
            transform: translateY(-5px);
        }
        .book-img {
            width: 100%;
            height: 150px;
            background: linear-gradient(45deg, #ff9a9e, #fad0c4);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            margin-bottom: 15px;
            color: #333;
            font-size: 1.2rem;
            text-align: center;
            padding: 10px;
            font-weight: bold;
        }
        .book-title {
            font-size: 1.1rem;
            text-align: center;
            font-weight: bold;
            color: var(--trading-gold);
        }
        .form-container {
            flex: 1;
            min-width: 300px;
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 20px;
            padding: 30px;
            box-shadow: 0 15px 25px rgba(0, 0, 0, 0.2);
            width: 100%;
            max-width: 500px;
            margin: 0 auto;
        }
        .form-container h2 {
            font-size: 2rem;
            margin-bottom: 25px;
            text-align: center;
            color: var(--trading-gold);
        }
        .form-group {
            margin-bottom: 20px;
        }
        .form-group label {
            display: block;
            margin-bottom: 8px;
            font-weight: bold;
            font-size: 1.1rem;
        }
        .form-group i {
            margin-right: 8px;
            color: var(--trading-gold);
        }
        input, select {
            width: 100%;
            padding: 15px;
            border: none;
            border-radius: 10px;
            background: rgba(255, 255, 255, 0.9);
            font-size: 1.1rem;
            color: #333;
        }
        input:focus, select:focus {
            outline: 3px solid var(--trading-gold);
        }
        .btn-trading {
            background: linear-gradient(to right, var(--trading-gold), #ff9900);
            color: var(--trading-dark);
            border: none;
            padding: 18px;
            border-radius: 10px;
            font-size: 1.2rem;
            font-weight: bold;
            cursor: pointer;
            width: 100%;
            transition: all 0.3s ease;
            box-shadow: 0 5px 15px rgba(255, 204, 0, 0.4);
            margin-top: 10px;
        }
        .btn-trading:hover {
            background: linear-gradient(to right, #ff9900, var(--trading-gold));
            transform: translateY(-3px);
            box-shadow: 0 8px 20px rgba(255, 204, 0, 0.6);
        }
        .spiritual-note {
            text-align: center;
            margin-top: 25px;
            font-style: italic;
            font-size: 1.1rem;
            color: var(--trading-gold);
        }
        /* Organizer Section */
        .organizer-section {
            padding: 2rem 0;
        }
        .organizer-container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .task-categories {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .category-card {
            background: white;
            border-radius: var(--border-radius);
            box-shadow: var(--box-shadow);
            padding: 1.5rem;
            text-align: center;
            transition: var(--transition);
            cursor: pointer;
        }
        .category-card:hover {
            transform: translateY(-5px);
        }
        .category-card.active {
            border: 2px solid var(--primary-color);
        }
        .category-icon {
            width: 60px;
            height: 60px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0 auto 1rem;
            font-size: 1.5rem;
            color: white;
        }
        .icon-spiritual {
            background: linear-gradient(135deg, #8e44ad, #9b59b6);
        }
        .icon-daily {
            background: linear-gradient(135deg, #3498db, #2980b9);
        }
        .icon-quranic {
            background: linear-gradient(135deg, #27ae60, #2ecc71);
        }
        .icon-social {
            background: linear-gradient(135deg, #e74c3c, #c0392b);
        }
        .category-title {
            font-size: 1.2rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
        }
        .category-count {
            font-size: 0.9rem;
            color: var(--gray-color);
        }
        .tasks-container {
            background: white;
            border-radius: var(--border-radius);
            box-shadow: var(--box-shadow);
            padding: 2rem;
        }
        .tasks-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid #eee;
        }
        .tasks-title {
            font-size: 1.5rem;
            font-weight: 600;
            color: var(--primary-color);
        }
        .add-task-btn {
            background: var(--primary-color);
            color: white;
            border: none;
            padding: 0.7rem 1.5rem;
            border-radius: 30px;
            cursor: pointer;
            font-weight: 600;
            display: flex;
            align-items: center;
            gap: 8px;
            transition: var(--transition);
        }
        .add-task-btn:hover {
            background: var(--secondary-color);
            transform: translateY(-2px);
        }
        .tasks-list {
            max-height: 500px;
            overflow-y: auto;
        }
        .task-item {
            display: flex;
            align-items: center;
            padding: 1rem;
            border-bottom: 1px solid #f0f0f0;
            transition: var(--transition);
        }
        .task-item:hover {
            background: #f9f9f9;
        }
        .task-item:last-child {
            border-bottom: none;
        }
        .task-checkbox {
            margin-right: 1rem;
            width: 18px;
            height: 18px;
            cursor: pointer;
        }
        .task-content {
            flex: 1;
        }
        .task-title {
            font-weight: 500;
            margin-bottom: 0.3rem;
        }
        .task-title.completed {
            text-decoration: line-through;
            color: var(--gray-color);
        }
        .task-meta {
            display: flex;
            gap: 1rem;
            font-size: 0.85rem;
            color: var(--gray-color);
        }
        .task-category {
            background: #f0f0f0;
            padding: 0.2rem 0.6rem;
            border-radius: 20px;
            font-size: 0.75rem;
        }
        .task-actions {
            display: flex;
            gap: 0.5rem;
        }
        .task-action-btn {
            background: none;
            border: none;
            cursor: pointer;
            color: var(--gray-color);
            transition: var(--transition);
        }
        .task-action-btn:hover {
            color: var(--primary-color);
        }
        .progress-container {
            background: white;
            border-radius: var(--border-radius);
            box-shadow: var(--box-shadow);
            padding: 2rem;
            margin-top: 2rem;
        }
        .progress-title {
            font-size: 1.5rem;
            font-weight: 600;
            color: var(--primary-color);
            margin-bottom: 1.5rem;
            text-align: center;
        }
        .progress-stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        .progress-stat {
            text-align: center;
            padding: 1.5rem;
            border-radius: var(--border-radius);
            background: #f8f9fa;
        }
        .progress-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--primary-color);
            margin-bottom: 0.5rem;
        }
        .progress-label {
            font-size: 0.9rem;
            color: var(--gray-color);
        }
        .progress-bar-container {
            margin-bottom: 1.5rem;
        }
        .progress-bar-label {
            display: flex;
            justify-content: space-between;
            margin-bottom: 0.5rem;
            font-size: 0.9rem;
        }
        .progress-bar {
            height: 10px;
            background: #e9ecef;
            border-radius: 5px;
            overflow: hidden;
        }
        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(135deg, var(--primary-color), var(--secondary-color));
            border-radius: 5px;
            transition: width 0.5s ease;
        }
        /* Modal Styles */
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }
        .modal.active {
            display: flex;
        }
        .modal-content {
            background: white;
            border-radius: var(--border-radius);
            padding: 2rem;
            width: 90%;
            max-width: 500px;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
        }
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid #eee;
        }
        .modal-title {
            font-size: 1.5rem;
            font-weight: 600;
            color: var(--primary-color);
        }
        .close-modal {
            background: none;
            border: none;
            font-size: 1.5rem;
            cursor: pointer;
            color: var(--gray-color);
        }
        .form-group {
            margin-bottom: 1.5rem;
        }
        .form-label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 500;
        }
        .form-input, .form-select {
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #ddd;
            border-radius: var(--border-radius);
            font-size: 1rem;
            transition: var(--transition);
        }
        .form-input:focus, .form-select:focus {
            outline: none;
            border-color: var(--primary-color);
            box-shadow: 0 0 0 3px rgba(26, 95, 122, 0.1);
        }
        .form-actions {
            display: flex;
            justify-content: flex-end;
            gap: 1rem;
            margin-top: 2rem;
        }
        .btn-cancel {
            background: #f8f9fa;
            color: var(--dark-color);
            border: 1px solid #ddd;
            padding: 0.7rem 1.5rem;
            border-radius: 30px;
            cursor: pointer;
            font-weight: 500;
            transition: var(--transition);
        }
        .btn-cancel:hover {
            background: #e9ecef;
        }
        .btn-save {
            background: var(--primary-color);
            color: white;
            border: none;
            padding: 0.7rem 1.5rem;
            border-radius: 30px;
            cursor: pointer;
            font-weight: 600;
            transition: var(--transition);
        }
        .btn-save:hover {
            background: var(--secondary-color);
            transform: translateY(-2px);
        }
        /* Empty State */
        .empty-state {
            text-align: center;
            padding: 3rem 1rem;
            color: var(--gray-color);
        }
        .empty-state i {
            font-size: 3rem;
            margin-bottom: 1rem;
            opacity: 0.5;
        }
        /* Calendar Section */
        .calendar-section {
            padding: 2rem 0;
        }
        .calendar-container {
            max-width: 800px;
            margin: 0 auto;
            border-radius: var(--border-radius);
            padding: 30px;
            box-shadow: var(--box-shadow);
            background: white;
        }
        .calendar-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 25px;
        }
        .calendar-header h2 {
            font-size: 2rem;
            color: var(--primary-color);
        }
        .calendar-header button {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            border: none;
            color: white;
            padding: 12px 18px;
            border-radius: 50%;
            font-size: 1.2rem;
            cursor: pointer;
            transition: var(--transition);
            box-shadow: 0 4px 10px rgba(0, 0, 0, 0.1);
        }
        .calendar-header button:hover {
            transform: scale(1.1);
        }
        .calendar-weekdays {
            display: grid;
            grid-template-columns: repeat(7, 1fr);
            text-align: center;
            font-weight: bold;
            margin-bottom: 15px;
            font-size: 1.2rem;
            color: var(--primary-color);
            border-bottom: 2px solid var(--secondary-color);
            padding-bottom: 10px;
        }
        .calendar-days {
            display: grid;
            grid-template-columns: repeat(7, 1fr);
            gap: 8px;
        }
        .calendar-days div {
            padding: 15px 5px;
            text-align: center;
            border-radius: 8px;
            cursor: pointer;
            font-size: 1.2rem;
            transition: var(--transition);
        }
        .calendar-days div:hover {
            background: rgba(21, 152, 149, 0.1);
            color: var(--primary-color);
            font-weight: bold;
        }
        .calendar-days div.today {
            background: linear-gradient(135deg, var(--primary-color) 0%, var(--secondary-color) 100%);
            color: white;
            font-weight: bold;
            box-shadow: 0 4px 10px rgba(21, 152, 149, 0.3);
        }
        .hijri-calendar {
            margin-top: 25px;
            text-align: center;
            padding: 15px;
            background: linear-gradient(135deg, #FF9800 0%, #FFB74D 100%);
            border-radius: var(--border-radius);
            color: white;
            font-size: 1.3rem;
            font-weight: 600;
            box-shadow: 0 4px 15px rgba(255, 152, 0, 0.3);
        }
        /* Footer */
        .app-footer {
            background: var(--dark-color);
            color: white;
            text-align: center;
            padding: 2rem;
            margin-top: 3rem;
            transition: opacity 0.3s ease, visibility 0.3s ease;
        }
        .footer-content {
            max-width: 800px;
            margin: 0 auto;
        }
        .footer-links {
            display: flex;
            justify-content: center;
            gap: 2rem;
            margin: 1.5rem 0;
            flex-wrap: wrap;
        }
        .footer-link {
            color: rgba(255, 255, 255, 0.7);
            text-decoration: none;
            transition: var(--transition);
        }
        .footer-link:hover {
            color: white;
        }
        .copyright {
            color: rgba(255, 255, 255, 0.6);
            font-size: 0.9rem;
            margin-top: 1rem;
        }
        /* Quran Study Section Overrides */
        #quran-study {
            padding: 0 !important;
            margin: 0 !important;
            height: 100vh;
            width: 100%;
            display: none;
        }
        #quran-study.active {
            display: block;
        }
        #quran-study iframe {
            width: 100vw;
            height: 100vh;
            border: none;
            margin: 0;
            padding: 0;
            display: block;
            position: fixed;
            top: 1;
            left: 0;
            z-index: 1;
        }
        body.quran-study-active .app-header {
            margin-bottom: auto;
        }

        body.quran-study-active .app-footer {
            display: none;
        }
        body.quran-study-active {
            overflow: hidden;
        }
        /* Responsive Design */
        @media (max-width: 992px) {
            .app-header {
                flex-direction: column;
                gap: 1rem;
                padding: 1rem;
            }
            .main-nav {
                flex-wrap: wrap;
                justify-content: center;
            }
            .hero-section h2 {
                font-size: 2rem;
            }
            .organizer-grid {
                grid-template-columns: 1fr;
            }
            .content {
                flex-direction: column;
            }
            .book-covers {
                grid-template-columns: repeat(2, 1fr);
            }
        }
        @media (max-width: 768px) {
            .main-content {
                padding: 1rem;
            }
            .cta-buttons {
                flex-direction: column;
                align-items: center;
            }
            .cta-btn {
                width: 100%;
                max-width: 300px;
                justify-content: center;
            }
            .user-actions {
                width: 100%;
                justify-content: center;
            }
            .footer-links {
                flex-direction: column;
                gap: 1rem;
            }
            .calendar-weekdays, .calendar-days {
                font-size: 0.9rem;
            }
            .calendar-days div {
                padding: 10px 2px;
            }
            .progress-stats {
                grid-template-columns: repeat(2, 1fr);
            }
            .form-actions {
                flex-direction: column;
            }
            .book-covers {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="app-container">
        <!-- Header -->
        <header class="app-header">
            <div class="logo">
                <div class="logo-icon">
                    <i class="fas fa-mosque"></i>
                </div>
                <h1 style="font-size: 18px">Muslim Organizer<br>Gen Z Study</h1>
            </div>
            <nav class="main-nav">
                <a href="#" class="nav-item active" data-section="home">
                    <i class="fas fa-home"></i>
                    <span>Home</span>
                </a>
                <a href="#" class="nav-item" data-section="organizer">
                    <i class="fas fa-tasks"></i>
                    <span>Organizer</span>
                </a>
                <a href="#" class="nav-item" data-section="calendar">
                    <i class="fas fa-calendar"></i>
                    <span>Calendar</span>
                </a>
                <a href="#" class="nav-item" data-section="quran-study">
                    <i class="fas fa-book-quran"></i>
                    <span>Quran & Hadith Study</span>
                </a>
                <div style="width: 1px; height: 32px; background: rgba(255, 255, 255, 0.3); margin: 0 0px; align-self: center;"></div>
                <a href="#" class="nav-item" data-section="gen-z">
                    <i class="fas fa-graduation-cap"></i>
                    <span>Gen Z Study</span>
                </a>
                <div style="width: 1px; height: 32px; background: rgba(255, 255, 255, 0.3); margin: 0 0px; align-self: center;"></div>
                <a href="#" class="nav-item" data-section="trading">
                    <i class="fas fa-chart-line"></i>
                    <span>Trading</span>
                </a>
                <div style="width: 1px; height: 32px; background: rgba(255, 255, 255, 0.3); margin: 0 5px; align-self: center;"></div>
            </nav>
            <div class="user-actions">
                <?php if ($logged_in): ?>
                    <span style="color: white; font-weight: 600;"><?= htmlspecialchars(ucfirst(explode('@', $email)[0])) ?></span>
                <?php else: ?>
                    <a href="login.php" class="btn btn-primary">
                        <i class="fas fa-user"></i>
                        <span>Login</span>
                    </a>
                <?php endif; ?>
            </div>
        </header>
        <!-- Main Content -->
        <main class="main-content">
            <!-- Home Section -->
            <section id="home" class="section active">
                <div class="hero-section">
                    <h2 style="margin-bottom: 0px;">
                        Koran & Hadith Technology<br>
                    </h2>
                    <p>Your comprehensive digital companion for spiritual growth, daily prayers, Quran study, and Islamic organization.</p>
                    <div class="cta-buttons">
                        <button class="cta-btn cta-primary">
                            <i class="fas fa-rocket"></i>
                            Get Started
                        </button>
                        <button class="cta-btn cta-secondary">
                            <i class="fas fa-play-circle"></i>
                            Watch Tutorial
                        </button>
                    </div>
                </div>
                <div class="dashboard-grid">
                    <div class="dashboard-card">
                        <div class="card-header">
                            <div class="card-icon icon-quran">
                                <i class="fas fa-book-quran"></i>
                            </div>
                            <h3 class="card-title">Quran & Islam Study</h3>
                        </div>
                        <p class="card-content">Access the Holy Quran with translations, Tafsir, Hadith collections, and Seerah resources.</p>
                        <div class="card-stats">
                            <div class="stat">
                                <div class="stat-value">114</div>
                                <div class="stat-label">Surahs</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">6,236</div>
                                <div class="stat-label">Verses</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">12</div>
                                <div class="stat-label">Books</div>
                            </div>
                        </div>
                    </div>
                    <div class="dashboard-card">
                        <div class="card-header">
                            <div class="card-icon icon-prayer">
                                <i class="fas fa-praying-hands"></i>
                            </div>
                            <h3 class="card-title">Prayer Tracker</h3>
                        </div>
                        <p class="card-content">Track your daily prayers, get accurate prayer times, and set reminders for your spiritual routine.</p>
                        <div class="card-stats">
                            <div class="stat">
                                <div class="stat-value">5/5</div>
                                <div class="stat-label">Today</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">92%</div>
                                <div class="stat-label">This Week</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">28</div>
                                <div class="stat-label">Streak</div>
                            </div>
                        </div>
                    </div>
                    <div class="dashboard-card">
                        <div class="card-header">
                            <div class="card-icon icon-organizer">
                                <i class="fas fa-tasks"></i>
                            </div>
                            <h3 class="card-title">Muslim Organizer</h3>
                        </div>
                        <p class="card-content">Plan your Islamic activities, set spiritual goals, and organize your religious studies.</p>
                        <div class="card-stats">
                            <div class="stat">
                                <div class="stat-value">7</div>
                                <div class="stat-label">Tasks</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">3</div>
                                <div class="stat-label">Goals</div>
                            </div>
                            <div class="stat">
                                <div class="stat-value">85%</div>
                                <div class="stat-label">Progress</div>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="prayer-widget">
                    <div class="widget-header">
                        <h3 class="widget-title">Today's Prayer Times</h3>
                        <div class="location">
                            <i class="fas fa-map-marker-alt"></i>
                            <span>Mecca, Saudi Arabia</span>
                        </div>
                    </div>
                    <div class="prayer-times">
                        <div class="prayer-time">
                            <div class="prayer-name">Fajr</div>
                            <div class="prayer-hour">5:24 AM</div>
                        </div>
                        <div class="prayer-time">
                            <div class="prayer-name">Sunrise</div>
                            <div class="prayer-hour">6:45 AM</div>
                        </div>
                        <div class="prayer-time active">
                            <div class="prayer-name">Dhuhr</div>
                            <div class="prayer-hour">12:30 PM</div>
                        </div>
                        <div class="prayer-time">
                            <div class="prayer-name">Asr</div>
                            <div class="prayer-hour">3:45 PM</div>
                        </div>
                        <div class="prayer-time">
                            <div class="prayer-name">Maghrib</div>
                            <div class="prayer-hour">6:15 PM</div>
                        </div>
                        <div class="prayer-time">
                            <div class="prayer-name">Isha</div>
                            <div class="prayer-hour">7:45 PM</div>
                        </div>
                    </div>
                </div>
                <section class="features-section">
                    <h2 class="section-title">Key Features</h2>
                    <div class="features-grid">
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-book-open"></i>
                            </div>
                            <h3 class="feature-title">Quran Reader</h3>
                            <p class="feature-desc">Read the Holy Quran with multiple translations, tafsir, and audio recitations.</p>
                        </div>
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-history"></i>
                            </div>
                            <h3 class="feature-title">Seerah & History</h3>
                            <p class="feature-desc">Learn about the Prophet's life and Islamic history through engaging content.</p>
                        </div>
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-language"></i>
                            </div>
                            <h3 class="feature-title">Learn Arabic</h3>
                            <p class="feature-desc">Master Arabic with interactive lessons designed for Quranic understanding.</p>
                        </div>
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-headphones"></i>
                            </div>
                            <h3 class="feature-title">Audio Library</h3>
                            <p class="feature-desc">Access a vast collection of Quran recitations, lectures, and Islamic podcasts.</p>
                        </div>
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-sticky-note"></i>
                            </div>
                            <h3 class="feature-title">Islamic Notes</h3>
                            <p class="feature-desc">Take and organize notes on your Islamic studies and reflections.</p>
                        </div>
                        <div class="feature-card">
                            <div class="feature-icon">
                                <i class="fas fa-chart-line"></i>
                            </div>
                            <h3 class="feature-title">Progress Tracking</h3>
                            <p class="feature-desc">Monitor your spiritual growth and religious activities with detailed analytics.</p>
                        </div>
                    </div>
                </section>
            </section>
            <!-- Organizer Section -->
            <section id="organizer" class="section">
                <div class="organizer-section">
                    <h2 class="section-title">Muslim Organizer</h2>
                    <p style="text-align: center; margin-bottom: 2rem; color: var(--gray-color);">Manage your Islamic tasks, goals, and spiritual activities</p>
                    <div class="organizer-container">
                        <!-- Task Categories -->
                        <div class="task-categories">
                            <div class="category-card active" data-category="all">
                                <div class="category-icon icon-spiritual">
                                    <i class="fas fa-pray"></i>
                                </div>
                                <div class="category-title">All Tasks</div>
                                <div class="category-count" id="all-count">0 tasks</div>
                            </div>
                            <div class="category-card" data-category="spiritual">
                                <div class="category-icon icon-spiritual">
                                    <i class="fas fa-hands-praying"></i>
                                </div>
                                <div class="category-title">Spiritual</div>
                                <div class="category-count" id="spiritual-count">0 tasks</div>
                            </div>
                            <div class="category-card" data-category="daily">
                                <div class="category-icon icon-daily">
                                    <i class="fas fa-sun"></i>
                                </div>
                                <div class="category-title">Daily Acts</div>
                                <div class="category-count" id="daily-count">0 tasks</div>
                            </div>
                            <div class="category-card" data-category="quranic">
                                <div class="category-icon icon-quranic">
                                    <i class="fas fa-book-quran"></i>
                                </div>
                                <div class="category-title">Quranic</div>
                                <div class="category-count" id="quranic-count">0 tasks</div>
                            </div>
                            <div class="category-card" data-category="social">
                                <div class="category-icon icon-social">
                                    <i class="fas fa-people-group"></i>
                                </div>
                                <div class="category-title">Social</div>
                                <div class="category-count" id="social-count">0 tasks</div>
                            </div>
                        </div>
                        <div class="organizer-grid">
                            <!-- Tasks Container -->
                            <div class="tasks-container">
                                <div class="tasks-header">
                                    <h3 class="tasks-title">My Tasks</h3>
                                    <button class="add-task-btn" id="add-task-btn">
                                        <i class="fas fa-plus"></i>
                                        Add Task
                                    </button>
                                </div>
                                <div class="tasks-list" id="tasks-list">
                                    <!-- Tasks will be dynamically added here -->
                                    <div class="empty-state" id="empty-state">
                                        <i class="fas fa-tasks"></i>
                                        <h3>No tasks yet</h3>
                                        <p>Add your first task to get started with organizing your Islamic activities</p>
                                    </div>
                                </div>
                            </div>
                            <!-- Progress Container -->
                            <div class="progress-container">
                                <h3 class="progress-title">Spiritual Progress</h3>
                                <div class="progress-stats">
                                    <div class="progress-stat">
                                        <div class="progress-value" id="completion-rate">0%</div>
                                        <div class="progress-label">Completion Rate</div>
                                    </div>
                                    <div class="progress-stat">
                                        <div class="progress-value" id="completed-tasks">0</div>
                                        <div class="progress-label">Tasks Completed</div>
                                    </div>
                                    <div class="progress-stat">
                                        <div class="progress-value" id="current-streak">0</div>
                                        <div class="progress-label">Current Streak</div>
                                    </div>
                                    <div class="progress-stat">
                                        <div class="progress-value" id="total-tasks">0</div>
                                        <div class="progress-label">Total Tasks</div>
                                    </div>
                                </div>
                                <div class="progress-bar-container">
                                    <div class="progress-bar-label">
                                        <span>Spiritual Tasks</span>
                                        <span id="spiritual-progress">0%</span>
                                    </div>
                                    <div class="progress-bar">
                                        <div class="progress-bar-fill" id="spiritual-bar" style="width: 0%"></div>
                                    </div>
                                </div>
                                <div class="progress-bar-container">
                                    <div class="progress-bar-label">
                                        <span>Daily Acts</span>
                                        <span id="daily-progress">0%</span>
                                    </div>
                                    <div class="progress-bar">
                                        <div class="progress-bar-fill" id="daily-bar" style="width: 0%"></div>
                                    </div>
                                </div>
                                <div class="progress-bar-container">
                                    <div class="progress-bar-label">
                                        <span>Quranic Tasks</span>
                                        <span id="quranic-progress">0%</span>
                                    </div>
                                    <div class="progress-bar">
                                        <div class="progress-bar-fill" id="quranic-bar" style="width: 0%"></div>
                                    </div>
                                </div>
                                <div class="progress-bar-container">
                                    <div class="progress-bar-label">
                                        <span>Social Tasks</span>
                                        <span id="social-progress">0%</span>
                                    </div>
                                    <div class="progress-bar">
                                        <div class="progress-bar-fill" id="social-bar" style="width: 0%"></div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </section>
            <!-- Calendar Section -->
            <section id="calendar" class="section">
                <div class="calendar-section">
                    <h2 class="section-title">Islamic Calendar</h2>
                    <div class="calendar-container">
                        <div class="calendar-header">
                            <button id="prev-month"><i class="fas fa-chevron-left"></i></button>
                            <h2 id="month-year">November 2025</h2>
                            <button id="next-month"><i class="fas fa-chevron-right"></i></button>
                        </div>
                        <div class="calendar-weekdays">
                            <div>Sun</div>
                            <div>Mon</div>
                            <div>Tue</div>
                            <div>Wed</div>
                            <div>Thu</div>
                            <div>Fri</div>
                            <div>Sat</div>
                        </div>
                        <div class="calendar-days" id="calendar-days">
                            <!-- Calendar days will be populated by JavaScript -->
                        </div>
                        <div class="hijri-calendar">
                            <h3>Hijri Date: <span id="hijri-date">1 Shawwal 1447</span></h3>
                        </div>
                    </div>
                </div>
            </section>
            <!-- Quran Study Section -->
            <section id="quran-study" class="section">
                <iframe src="./quran-study/index.php" id="frame" frameborder="0"></iframe>
                <script>
                    var _frame = document.getElementById("frame");
                    _frame.contentWindow.location.href = _frame.src;
                </script>
            </section>
            <!-- Gen Z Section -->
            <section id="gen-z" class="section">
                <div class="gen-z-container">
                    <h2 class="section-title">Gen Z Online Study</h2>
                    <p class="tagline">Technology • Morality • Freelance Business • Creative Automation • Trading Simulation</p>
                    <div class="domains">
                        <div class="domain domain-tech">
                            <h2>Technology & Innovation</h2>
                            <p>Explore Gen Z adoption of AI, VR/AR, automation tools and digital ecosystems.</p>
                            <ul>
                                <li>Which emerging technologies excite Gen Z most?</li>
                                <li>How do they use AI for creativity and productivity?</li>
                                <li>What motivates adoption of new digital tools?</li>
                            </ul>
                        </div>
                        <div class="domain domain-morality">
                            <h2>Morality & Ethics</h2>
                            <p>Understand Gen Z values around ethics, social responsibility, and digital integrity.</p>
                            <ul>
                                <li>How important is a company's moral stance in purchasing decisions?</li>
                                <li>Which causes matter most (environment, justice, mental health)?</li>
                                <li>How concerned are they about privacy and AI misuse?</li>
                            </ul>
                        </div>
                        <div class="domain domain-freelance">
                            <h2>Freelance & Digital Business</h2>
                            <p>Measure Gen Z participation in the gig economy and digital entrepreneurship.</p>
                            <ul>
                                <li>What percentage earns from freelance or gig work?</li>
                                <li>Which skills do they use or want to learn?</li>
                                <li>What are the biggest barriers to starting a business?</li>
                            </ul>
                        </div>
                        <div class="domain domain-creative">
                            <h2>Creative Automation</h2>
                            <p>Examine Gen Z's use of AI design tools and art automation.</p>
                            <ul>
                                <li>How often do they use AI to create art, music, or code?</li>
                                <li>Do they view automation as empowering or replacing creativity?</li>
                                <li>What features matter most in new apps?</li>
                            </ul>
                        </div>
                        <div class="domain domain-trading">
                            <h2>Trading Simulation</h2>
                            <p>Analyze Gen Z's involvement with simulated trading and financial behavior.</p>
                            <ul>
                                <li>Have they tried simulated trading before investing real money?</li>
                                <li>What platforms or assets do they prefer?</li>
                                <li>Who influences their financial decisions?</li>
                            </ul>
                        </div>
                    </div>
                    <div class="cta-section">
                        <h2>Your Voice Matters</h2>
                        <p>We're studying how our generation works, creates, earns, and thinks — from AI art to freelancing to ethics.</p>
                        <p>If you've ever used AI for work or fun, taken freelance gigs, designed with automation, tried trading, or thought about digital ethics — this study is for YOU.</p>
                        <a href="#" class="cta-button">Take the Survey Now</a>
                    </div>
                    <section class="research-objectives">
                        <h2>Research Objectives</h2>
                        <div class="objectives-grid">
                            <div class="objective">
                                <h3>Technology Adoption</h3>
                                <p>Understand Gen Z's use of emerging technologies and digital tools for creativity and productivity.</p>
                            </div>
                            <div class="objective">
                                <h3>Digital Entrepreneurship</h3>
                                <p>Measure participation in freelancing and identify motivations and barriers to digital business.</p>
                            </div>
                            <div class="objective">
                                <h3>Financial Behavior</h3>
                                <p>Analyze involvement with trading simulations and real-money investing.</p>
                            </div>
                            <div class="objective">
                                <h3>Creative Automation</h3>
                                <p>Examine use of AI design tools and perspectives on creativity in the age of automation.</p>
                            </div>
                            <div class="objective">
                                <h3>Digital Ethics</h3>
                                <p>Explore values related to ethics, social responsibility, and personal integrity online.</p>
                            </div>
                        </div>
                    </section>
                    <footer>
                        <p>Gen Z Online Study &copy; 2023</p>
                        <div class="hashtags">
                            #GenZStudy #FutureOfWork #AIEthics #FreelanceLife #DigitalGeneration #Zoomers
                        </div>
                    </footer>
                </div>
            </section>
            <!-- Trading Section -->
            <section id="trading" class="section">
                <div class="trading-container">
                    <div class="trading-header">
                        <h1>Marketplace & Trading</h1>
                        <div class="subtitle">Learn Stock and Forex Trading</div>
                        <p>Explore ethical trading opportunities while strengthening your spiritual journey</p>
                    </div>
                    <div class="promo-text">
                        <p><strong>Trading manual atau otomatis dalam hitungan menit, bukan jam.</strong> Kuasai aturan emas: tunggu momen terbaik, bertindak tegas, dan trading hanya 36 menit sehari—dengan risiko terbatas 0,5%. <strong>Daftar Sekarang.</strong></p>
                        <p><strong>Trade manually or automatically in minutes, not hours.</strong> Master the golden rule: wait for the perfect setup, act decisively, and trade just 36 minutes a day—risk capped at 0.5%. <strong>Register Now.</strong></p>
                    </div>
                    <div class="content">
                        <div class="book-covers">
                            <div class="book-cover">
                                <div class="book-img">Forex Fundamentals</div>
                                <div class="book-title">Master the Forex Market</div>
                            </div>
                            <div class="book-cover">
                                <div class="book-img">Stock Trading 101</div>
                                <div class="book-title">Beginner's Guide to Stocks</div>
                            </div>
                            <div class="book-cover">
                                <div class="book-img">Technical Analysis</div>
                                <div class="book-title">Charts & Indicators</div>
                            </div>
                            <div class="book-cover">
                                <div class="book-img">Risk Management</div>
                                <div class="book-title">Protect Your Capital</div>
                            </div>
                            <div class="book-cover">
                                <div class="book-img">Trading Psychology</div>
                                <div class="book-title">Mind Over Markets</div>
                            </div>
                            <div class="book-cover">
                                <div class="book-img">Ethical Investing</div>
                                <div class="book-title">Values-Based Strategies</div>
                            </div>
                        </div>
                        <div class="form-container">
                            <h2>Register Now</h2>
                            <form id="registrationForm">
                                <div class="form-group">
                                    <label for="fullname"><i class="fas fa-user"></i> Full Name</label>
                                    <input type="text" id="fullname" name="fullname" required>
                                </div>
                                <div class="form-group">
                                    <label for="email"><i class="fas fa-envelope"></i> Email Address</label>
                                    <input type="email" id="email" name="email" required>
                                </div>
                                <div class="form-group">
                                    <label for="phone"><i class="fas fa-phone"></i> Phone Number</label>
                                    <input type="tel" id="phone" name="phone" required>
                                </div>
                                <div class="form-group">
                                    <label for="experience"><i class="fas fa-chart-line"></i> Trading Experience</label>
                                    <select id="experience" name="experience" required>
                                        <option value="">Select your experience level</option>
                                        <option value="beginner">Beginner</option>
                                        <option value="intermediate">Intermediate</option>
                                        <option value="advanced">Advanced</option>
                                    </select>
                                </div>
                                <div class="form-group">
                                    <label for="interest"><i class="fas fa-star"></i> Primary Interest</label>
                                    <select id="interest" name="interest" required>
                                        <option value="">Select your interest</option>
                                        <option value="stocks">Stock Trading</option>
                                        <option value="forex">Forex Trading</option>
                                        <option value="both">Both</option>
                                    </select>
                                </div>
                                <button type="submit" class="btn-trading">Register Now <i class="fas fa-arrow-right"></i></button>
                                <div class="spiritual-note">
                                    <p>Align your financial goals with your spiritual values</p>
                                </div>
                            </form>
                        </div>
                    </div>
                </div>
            </section>
        </main>
        <!-- Task Modal -->
        <div class="modal" id="task-modal">
            <div class="modal-content">
                <div class="modal-header">
                    <h3 class="modal-title" id="modal-title">Add New Task</h3>
                    <button class="close-modal" id="close-modal">&times;</button>
                </div>
                <form id="task-form">
                    <input type="hidden" id="task-id">
                    <div class="form-group">
                        <label for="task-title" class="form-label">Task Title</label>
                        <input type="text" id="task-title" class="form-input" placeholder="Enter task title" required>
                    </div>
                    <div class="form-group">
                        <label for="task-category" class="form-label">Category</label>
                        <select id="task-category" class="form-select" required>
                            <option value="">Select a category</option>
                            <option value="spiritual">Spiritual</option>
                            <option value="daily">Daily Acts</option>
                            <option value="quranic">Quranic</option>
                            <option value="social">Social</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label for="task-due" class="form-label">Due Date (Optional)</label>
                        <input type="date" id="task-due" class="form-input">
                    </div>
                    <div class="form-actions">
                        <button type="button" class="btn-cancel" id="cancel-task">Cancel</button>
                        <button type="submit" class="btn-save">Save Task</button>
                    </div>
                </form>
            </div>
        </div>
        <!-- Footer -->
        <footer class="app-footer">
            <div class="footer-content">
                <h3>Muslim Organizer & Gen Z Study</h3>
                <p>Your comprehensive digital companion for spiritual growth, Islamic organization, and modern learning</p>
                <div class="footer-links">
                    <a href="#" class="footer-link">About Us</a>
                    <a href="#" class="footer-link">Privacy Policy</a>
                    <a href="#" class="footer-link">Terms of Service</a>
                    <a href="#" class="footer-link">Contact</a>
                    <a href="#" class="footer-link">Support</a>
                </div>
                <div class="copyright">
                    © 2023 Muslim Organizer - www.NabiMuhammad.com/Muslim-Organizer & Gen Z Study
                </div>
            </div>
        </footer>
    </div>
    <script>
        // Task Management with Local Storage
        class TaskManager {
            constructor() {
                this.tasks = this.loadTasks();
                this.currentFilter = 'all';
                this.editingTaskId = null;
                this.init();
            }
            init() {
                this.renderTasks();
                this.updateStats();
                this.setupEventListeners();
            }
            loadTasks() {
                const tasks = localStorage.getItem('muslimOrganizerTasks');
                return tasks ? JSON.parse(tasks) : [];
            }
            saveTasks() {
                localStorage.setItem('muslimOrganizerTasks', JSON.stringify(this.tasks));
            }
            addTask(title, category, dueDate = null) {
                const task = {
                    id: Date.now().toString(),
                    title,
                    category,
                    dueDate,
                    completed: false,
                    createdAt: new Date().toISOString()
                };
                this.tasks.push(task);
                this.saveTasks();
                this.renderTasks();
                this.updateStats();
            }
            editTask(id, title, category, dueDate) {
                const task = this.tasks.find(t => t.id === id);
                if (task) {
                    task.title = title;
                    task.category = category;
                    task.dueDate = dueDate;
                    this.saveTasks();
                    this.renderTasks();
                    this.updateStats();
                }
            }
            deleteTask(id) {
                this.tasks = this.tasks.filter(t => t.id !== id);
                this.saveTasks();
                this.renderTasks();
                this.updateStats();
            }
            toggleTask(id) {
                const task = this.tasks.find(t => t.id === id);
                if (task) {
                    task.completed = !task.completed;
                    this.saveTasks();
                    this.renderTasks();
                    this.updateStats();
                }
            }
            getFilteredTasks() {
                if (this.currentFilter === 'all') {
                    return this.tasks;
                }
                return this.tasks.filter(task => task.category === this.currentFilter);
            }
            renderTasks() {
                const tasksList = document.getElementById('tasks-list');
                const emptyState = document.getElementById('empty-state');
                const filteredTasks = this.getFilteredTasks();
                // Clear the tasks list
                tasksList.innerHTML = '';
                if (filteredTasks.length === 0) {
                    // Show empty state if no tasks
                    tasksList.appendChild(emptyState);
                    if (emptyState) emptyState.style.display = 'block';
                    return;
                }
                // Hide empty state
                if (emptyState) emptyState.style.display = 'none';
                // Add tasks to the list
                filteredTasks.forEach(task => {
                    const taskItem = document.createElement('div');
                    taskItem.className = 'task-item';
                    taskItem.innerHTML = `
                        <input type="checkbox" class="task-checkbox" ${task.completed ? 'checked' : ''} data-id="${task.id}">
                        <div class="task-content">
                            <div class="task-title ${task.completed ? 'completed' : ''}">${task.title}</div>
                            <div class="task-meta">
                                <span class="task-category">${this.formatCategory(task.category)}</span>
                                <span>${task.dueDate ? this.formatDate(task.dueDate) : 'No due date'}</span>
                            </div>
                        </div>
                        <div class="task-actions">
                            <button class="task-action-btn edit-task" data-id="${task.id}"><i class="fas fa-edit"></i></button>
                            <button class="task-action-btn delete-task" data-id="${task.id}"><i class="fas fa-trash"></i></button>
                        </div>
                    `;
                    tasksList.appendChild(taskItem);
                });
                // Add event listeners to the new elements
                this.attachTaskEventListeners();
            }
            attachTaskEventListeners() {
                // Checkboxes
                document.querySelectorAll('.task-checkbox').forEach(checkbox => {
                    checkbox.addEventListener('change', (e) => {
                        this.toggleTask(e.target.dataset.id);
                    });
                });
                // Edit buttons
                document.querySelectorAll('.edit-task').forEach(button => {
                    button.addEventListener('click', (e) => {
                        this.openEditModal(e.target.closest('button').dataset.id);
                    });
                });
                // Delete buttons
                document.querySelectorAll('.delete-task').forEach(button => {
                    button.addEventListener('click', (e) => {
                        if (confirm('Are you sure you want to delete this task?')) {
                            this.deleteTask(e.target.closest('button').dataset.id);
                        }
                    });
                });
            }
            formatCategory(category) {
                const categories = {
                    'spiritual': 'Spiritual',
                    'daily': 'Daily Acts',
                    'quranic': 'Quranic',
                    'social': 'Social'
                };
                return categories[category] || category;
            }
            formatDate(dateString) {
                const options = { year: 'numeric', month: 'short', day: 'numeric' };
                return new Date(dateString).toLocaleDateString(undefined, options);
            }
            updateStats() {
                // Update category counts
                const categories = ['all', 'spiritual', 'daily', 'quranic', 'social'];
                categories.forEach(category => {
                    const count = category === 'all'
                        ? this.tasks.length
                        : this.tasks.filter(t => t.category === category).length;
                    const countElement = document.getElementById(`${category}-count`);
                    if (countElement) {
                        countElement.textContent = `${count} ${count === 1 ? 'task' : 'tasks'}`;
                    }
                });
                // Update progress stats
                const totalTasks = this.tasks.length;
                const completedTasks = this.tasks.filter(t => t.completed).length;
                const completionRate = totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0;
                const completionRateElement = document.getElementById('completion-rate');
                const completedTasksElement = document.getElementById('completed-tasks');
                const totalTasksElement = document.getElementById('total-tasks');
                const streakElement = document.getElementById('current-streak');
                if (completionRateElement) completionRateElement.textContent = `${completionRate}%`;
                if (completedTasksElement) completedTasksElement.textContent = completedTasks;
                if (totalTasksElement) totalTasksElement.textContent = totalTasks;
                // Calculate streak
                const currentStreak = this.calculateStreak();
                if (streakElement) streakElement.textContent = currentStreak;
                // Update progress bars for each category
                categories.filter(c => c !== 'all').forEach(category => {
                    const categoryTasks = this.tasks.filter(t => t.category === category);
                    const completedCategoryTasks = categoryTasks.filter(t => t.completed).length;
                    const categoryProgress = categoryTasks.length > 0
                        ? Math.round((completedCategoryTasks / categoryTasks.length) * 100)
                        : 0;
                    const progressElement = document.getElementById(`${category}-progress`);
                    const barElement = document.getElementById(`${category}-bar`);
                    if (progressElement) progressElement.textContent = `${categoryProgress}%`;
                    if (barElement) barElement.style.width = `${categoryProgress}%`;
                });
            }
            calculateStreak() {
                // Simplified streak calculation - in a real app, you'd track daily completions
                if (this.tasks.length === 0) return 0;
                // Check if any task was completed today
                const today = new Date().toDateString();
                const completedToday = this.tasks.some(task => {
                    if (!task.completed) return false;
                    // Check if task was completed today
                    const completedDate = new Date(task.createdAt).toDateString();
                    return completedDate === today;
                });
                // For demo purposes, return a streak based on today's completion
                return completedToday ? Math.min(7, Math.floor(Math.random() * 7) + 1) : 0;
            }
            openAddModal() {
                document.getElementById('modal-title').textContent = 'Add New Task';
                document.getElementById('task-form').reset();
                document.getElementById('task-id').value = '';
                this.editingTaskId = null;
                document.getElementById('task-modal').classList.add('active');
            }
            openEditModal(id) {
                const task = this.tasks.find(t => t.id === id);
                if (task) {
                    document.getElementById('modal-title').textContent = 'Edit Task';
                    document.getElementById('task-id').value = task.id;
                    document.getElementById('task-title').value = task.title;
                    document.getElementById('task-category').value = task.category;
                    document.getElementById('task-due').value = task.dueDate || '';
                    this.editingTaskId = id;
                    document.getElementById('task-modal').classList.add('active');
                }
            }
            closeModal() {
                document.getElementById('task-modal').classList.remove('active');
                this.editingTaskId = null;
            }
            handleFormSubmit(e) {
                e.preventDefault();
                const title = document.getElementById('task-title').value.trim();
                const category = document.getElementById('task-category').value;
                const dueDate = document.getElementById('task-due').value || null;
                if (!title || !category) {
                    alert('Please fill in all required fields');
                    return;
                }
                if (this.editingTaskId) {
                    this.editTask(this.editingTaskId, title, category, dueDate);
                } else {
                    this.addTask(title, category, dueDate);
                }
                this.closeModal();
            }
            setupEventListeners() {
                // Add task button
                document.getElementById('add-task-btn').addEventListener('click', () => {
                    this.openAddModal();
                });
                // Category filters
                document.querySelectorAll('.category-card').forEach(card => {
                    card.addEventListener('click', (e) => {
                        document.querySelectorAll('.category-card').forEach(c => c.classList.remove('active'));
                        card.classList.add('active');
                        this.currentFilter = card.dataset.category;
                        this.renderTasks();
                    });
                });
                // Modal controls
                document.getElementById('close-modal').addEventListener('click', () => {
                    this.closeModal();
                });
                document.getElementById('cancel-task').addEventListener('click', () => {
                    this.closeModal();
                });
                document.getElementById('task-form').addEventListener('submit', (e) => {
                    this.handleFormSubmit(e);
                });
                // Close modal when clicking outside
                document.getElementById('task-modal').addEventListener('click', (e) => {
                    if (e.target.id === 'task-modal') {
                        this.closeModal();
                    }
                });
            }
        }
        // Calendar and Trading functionality
        document.addEventListener('DOMContentLoaded', function() {
            // Initialize Task Manager
            const taskManager = new TaskManager();
            // Navigation
            const navItems = document.querySelectorAll('.nav-item');
            const sections = document.querySelectorAll('.section');
            const appContainer = document.querySelector('.app-container');
            const footer = document.querySelector('.app-footer');

            function updateActiveSection(targetId) {
                // Update active nav item
                navItems.forEach(nav => nav.classList.remove('active'));
                const activeNavItem = document.querySelector(`.nav-item[data-section="${targetId}"]`);
                if (activeNavItem) activeNavItem.classList.add('active');

                // Show corresponding section
                sections.forEach(section => {
                    section.classList.remove('active');
                    if (section.id === targetId) {
                        section.classList.add('active');
                    }
                });

                // Toggle body class for Quran Study
                if (targetId === 'quran-study') {
                    document.body.classList.add('quran-study-active');
                } else {
                    document.body.classList.remove('quran-study-active');
                }
            }

            navItems.forEach(item => {
                item.addEventListener('click', function(e) {
                    e.preventDefault();
                    const targetId = this.getAttribute('data-section');
                    updateActiveSection(targetId);
                });
            });

            // Update active prayer time based on current time
            function updateActivePrayer() {
                const prayerTimes = document.querySelectorAll('.prayer-time');
                // This is a simplified logic - in a real app you would use accurate prayer times
                prayerTimes.forEach(prayer => {
                    prayer.classList.remove('active');
                });
                // For demo purposes, always activate Dhuhr
                document.querySelectorAll('.prayer-time')[2].classList.add('active');
            }
            updateActivePrayer();

            // Trading form submission
            const tradingForm = document.getElementById('registrationForm');
            if (tradingForm) {
                tradingForm.addEventListener('submit', function(e) {
                    e.preventDefault();
                    alert('Thank you for registering! You will be redirected to the confirmation page.');
                });
            }

            // Calendar functionality
            const monthYearElement = document.getElementById('month-year');
            const calendarDaysElement = document.getElementById('calendar-days');
            const prevMonthButton = document.getElementById('prev-month');
            const nextMonthButton = document.getElementById('next-month');
            const hijriDateElement = document.getElementById('hijri-date');
            let currentDate = new Date();

            function renderCalendar() {
                const year = currentDate.getFullYear();
                const month = currentDate.getMonth();
                const firstDay = new Date(year, month, 1);
                const lastDay = new Date(year, month + 1, 0);
                const daysInMonth = lastDay.getDate();
                const startingDay = firstDay.getDay();
                const today = new Date();
                const monthNames = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
                monthYearElement.textContent = `${monthNames[month]} ${year}`;
                calendarDaysElement.innerHTML = '';
                // Add empty cells for days before the first day of the month
                for (let i = 0; i < startingDay; i++) {
                    const emptyDiv = document.createElement('div');
                    calendarDaysElement.appendChild(emptyDiv);
                }
                // Add cells for each day of the month
                for (let i = 1; i <= daysInMonth; i++) {
                    const dayElement = document.createElement('div');
                    dayElement.textContent = i;
                    if (year === today.getFullYear() && month === today.getMonth() && i === today.getDate()) {
                        dayElement.classList.add('today');
                    }
                    calendarDaysElement.appendChild(dayElement);
                }
                // Set a default Hijri date for demo purposes
                hijriDateElement.textContent = "1 Shawwal 1447";
            }

            prevMonthButton.addEventListener('click', function() {
                currentDate.setMonth(currentDate.getMonth() - 1);
                renderCalendar();
            });
            nextMonthButton.addEventListener('click', function() {
                currentDate.setMonth(currentDate.getMonth() + 1);
                renderCalendar();
            });
            renderCalendar();
        });
    </script>
</body>
</html>
