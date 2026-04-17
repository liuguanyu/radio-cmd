"""
Advanced Radio.cn sniffer with DOM interaction
Clicks on stations to trigger playback and capture the actual streaming URLs
"""

import time
import json
import os
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.service import Service
from selenium.common.exceptions import TimeoutException, NoSuchElementException

def setup_driver():
    """Setup Chrome driver with network interception"""
    chrome_options = Options()

    # For debugging, show browser
    chrome_options.add_argument("--headless=false")

    chrome_options.add_argument("--no-sandbox")
    chrome_options.add_argument("--disable-dev-shm-usage")
    chrome_options.add_argument("--disable-gpu")
    chrome_options.add_argument("--window-size=1920,1080")

    # Enable DevTools Protocol to intercept network requests
    chrome_options.set_capability("goog:loggingPrefs", {"performance": "ALL"})

    return webdriver.Chrome(options=chrome_options)

def extract_station_list(driver):
    """Extract all radio stations from the page"""
    stations = []

    print("Extracting station list...")

    # Wait for station elements to load
    try:
        WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, ".station-item, .radio-station-item"))
        )
    except TimeoutException:
        print("Warning: Could not find station items")

    # Try different selectors for station items - expanded for better coverage
    selectors = [
        ".station-item",
        ".radio-station-item",
        "[data-station-id]",
        ".station-list .item",
        ".station-card",
        ".radio-card",
        ".item",
        "[class*='station']",
        "[class*='radio']",
        "div[class^='station']",
        "div[class^='radio']"
    ]

    for selector in selectors:
        elements = driver.find_elements(By.CSS_SELECTOR, selector)
        if elements:
            print(f"Found {len(elements)} items with selector: {selector}")
            for elem in elements:
                try:
                    # Try different class names for station name
                    name_selectors = [".station-name", ".name", ".title", ".radio-name"]
                    name = None

                    for name_sel in name_selectors:
                        try:
                            name_elem = elem.find_element(By.CSS_SELECTOR, name_sel)
                            name = name_elem.text.strip()
                            break
                        except NoSuchElementException:
                            continue

                    if not name:
                        # Use the whole element text
                        name = elem.text.strip().split('\n')[0]

                    if name:
                        station_id = elem.get_attribute("data-station-id") or elem.get_attribute("id") or ""
                        stations.append({
                            "name": name,
                            "id": station_id,
                            "element_selector": selector
                        })
                except Exception as e:
                    continue
            break

    return stations

def click_station_and_capture(driver, station):
    """Click on a station and capture network traffic"""
    print(f"\nClicking on station: {station['name']}")

    # Find the element again (in case page changed)
    try:
        element = driver.find_element(By.CSS_SELECTOR, station["element_selector"])
        element.click()
        print("Station clicked, waiting for network requests...")
        time.sleep(3)  # Wait for playback to start

        # Get performance logs
        logs = driver.get_log("performance")

        playback_urls = []

        # Parse logs for playback URLs
        for log in logs:
            try:
                message = json.loads(log["message"])

                if message["message"]["method"] == "Network.requestWillBeSent":
                    url = message["message"]["params"]["request"]["url"]

                    # Common patterns for radio streams
                    if "radio.cn" in url and any(ext in url.lower() for ext in [".m3u8", ".mp3", ".m3u", ".wav", ".aac"]):
                        playback_urls.append(url)
                    elif "stream" in url.lower() and "radio.cn" in url:
                        playback_urls.append(url)
                    elif "audio" in url.lower() and "radio.cn" in url:
                        playback_urls.append(url)
            except:
                continue

        return list(set(playback_urls))  # Deduplicate

    except Exception as e:
        print(f"Error clicking station: {e}")
        return []

def sniff_radio_stations():
    """Main sniffer function"""
    driver = setup_driver()

    try:
        # Navigate to page
        print("Navigating to radio.cn...")
        driver.get("https://www.radio.cn/pc-portal/erji/radioStation.html")

        # Wait for page load - extended wait time for JS to execute
        WebDriverWait(driver, 20).until(
            EC.presence_of_element_located((By.TAG_NAME, "body"))
        )
        # Wait longer for dynamic content to load
        time.sleep(8)

        # Extract station list
        stations = extract_station_list(driver)

        print(f"\nFound {len(stations)} stations total")
        if stations:
            print("First few stations:")
            for s in stations[:5]:
                print(f"  - {s['name']} (ID: {s['id']})")

        # For each station, click and capture playback URL
        results = []
        for i, station in enumerate(stations):
            print(f"\n[{i+1}/{len(stations)}] Processing: {station['name']}")

            # Skip if already have an ID to try
            if station["id"]:
                urls = click_station_and_capture(driver, station)
                if urls:
                    station["playback_urls"] = urls
                    results.append(station)
                    print(f"  Found {len(urls)} playback URLs:")
                    for url in urls:
                        print(f"    - {url}")
                else:
                    print("  No playback URLs found")

            # Refresh page occasionally
            if i > 0 and i % 5 == 0:
                print("Refreshing page to reset state...")
                driver.refresh()
                time.sleep(3)

        # Save results
        output = {
            "analysis_date": time.strftime("%Y-%m-%d %H:%M:%S"),
            "total_stations": len(results),
            "stations": results
        }

        with open("radio_station_data.json", "w", encoding="utf-8") as f:
            json.dump(output, f, ensure_ascii=False, indent=2)

        print(f"\n=== SUMMARY ===")
        print(f"Total stations with playback URLs: {len(results)}")
        for station in results:
            print(f"\n{station['name']}:")
            for url in station.get("playback_urls", []):
                print(f"  {url}")

        print(f"\nComplete data saved to radio_station_data.json")

        return output

    finally:
        driver.quit()

if __name__ == "__main__":
    sniff_radio_stations()
