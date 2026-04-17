"""
Simple network capture script for radio.cn
Focuses on capturing API requests and analyzing page structure
"""

import time
import json
from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC

def capture_network_traffic():
    """Capture all network traffic from radio.cn"""

    chrome_options = Options()
    # Run with visible browser for debugging
    # chrome_options.add_argument("--headless")
    chrome_options.add_argument("--no-sandbox")
    chrome_options.add_argument("--disable-dev-shm-usage")
    chrome_options.set_capability("goog:loggingPrefs", {"performance": "ALL"})

    driver = webdriver.Chrome(options=chrome_options)

    try:
        print("Navigating to radio.cn...")
        driver.get("https://www.radio.cn/pc-portal/erji/radioStation.html")

        # Wait for page to fully load
        WebDriverWait(driver, 20).until(
            EC.presence_of_element_located((By.TAG_NAME, "body"))
        )
        print("Page loaded, waiting for JS to execute...")
        time.sleep(10)

        # Get all performance logs
        logs = driver.get_log("performance")
        print(f"\nCaptured {len(logs)} network events")

        # Parse and categorize requests
        api_requests = []
        media_requests = []

        for log in logs:
            try:
                message = json.loads(log["message"])
                method = message["message"]["method"]

                if method == "Network.requestWillBeSent":
                    url = message["message"]["params"]["request"]["url"]

                    # Filter for radio.cn requests
                    if "radio.cn" in url or "cnr.cn" in url:
                        # Categorize
                        if any(ext in url.lower() for ext in [".json", "api", "list"]):
                            api_requests.append(url)
                        elif any(ext in url.lower() for ext in [".m3u8", ".mp3", ".m3u", "stream", "audio"]):
                            media_requests.append(url)
                        else:
                            api_requests.append(url)
            except:
                continue

        print(f"\n=== API Requests ({len(api_requests)}) ===")
        for url in list(set(api_requests))[:20]:
            print(f"  {url}")

        print(f"\n=== Media Requests ({len(media_requests)}) ===")
        for url in list(set(media_requests))[:20]:
            print(f"  {url}")

        # Get page source and look for radio stations
        print("\n=== Page Structure ===")
        page_source = driver.page_source

        # Try to find all elements with text content
        try:
            # Get all clickable elements
            clickable_elements = driver.find_elements(By.CSS_SELECTOR, "[onclick], a[href], button")
            print(f"\nFound {len(clickable_elements)} clickable elements")

            # Look for elements with radio station names
            common_names = ["之声", "广播", "音乐", "新闻", "交通", "经济", "文艺", "都市"]
            potential_stations = []

            for elem in clickable_elements:
                text = elem.text.strip()
                if text and len(text) < 20:  # Likely a station name
                    for name_part in common_names:
                        if name_part in text:
                            potential_stations.append({
                                "text": text,
                                "class": elem.get_attribute("class"),
                                "id": elem.get_attribute("id"),
                                "onclick": elem.get_attribute("onclick"),
                                "href": elem.get_attribute("href")
                            })
                            break

            print(f"\n=== Potential Stations ({len(potential_stations)}) ===")
            for station in potential_stations[:15]:
                print(f"  {station['text']}")
                print(f"    class: {station['class']}")
                print(f"    id: {station['id']}")
                print(f"    onclick: {station['onclick']}")
                print(f"    href: {station['href']}")

        except Exception as e:
            print(f"Error finding elements: {e}")

        # Save captured data
        output = {
            "api_requests": list(set(api_requests)),
            "media_requests": list(set(media_requests)),
            "potential_stations": potential_stations if 'potential_stations' in locals() else []
        }

        with open("network_capture.json", "w", encoding="utf-8") as f:
            json.dump(output, f, ensure_ascii=False, indent=2)

        print(f"\nData saved to network_capture.json")

        # Keep browser open for manual inspection
        print("\nBrowser will stay open for 30 seconds for manual inspection...")
        time.sleep(30)

        return output

    finally:
        driver.quit()

if __name__ == "__main__":
    capture_network_traffic()
