#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

# Dependencies: selenium, python-dotenv

import dotenv
import os
import time
from selenium import webdriver
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.common.by import By
from selenium.common.exceptions import TimeoutException, NoSuchElementException
from selenium.webdriver.chrome.options import Options


dotenv.load_dotenv()

PROD_RS_URL="https://registry-rs.edgeorchestration.intel.com/oauth/login"
USERNAME=os.getenv("RS_USERNAME")
PASSWORD=os.getenv("RS_PASSWORD")
LOGIN_URL=os.getenv("RS_LOGIN_URL", PROD_RS_URL)
DEBUG=os.getenv("DEBUG")

azure_ad_b2c=os.getenv("AZURE_AD_B2C", "false")
azure_ad_b2c = azure_ad_b2c.lower() == "true"

chrome_options = Options()
chrome_options.add_argument("--headless=new")
chrome_options.add_argument('--no-sandbox')
chrome_options.add_argument("--remote-debugging-pipe")
#chrome_options.add_argument("--remote-debugging-port=9222")
chrome_options.add_argument('--proxy-server=http://proxy-us.intel.com:912')

driver = webdriver.Chrome(chrome_options)
action_chain = webdriver.ActionChains(driver)

def debug_print(msg):
  if DEBUG:
    print(msg)

def wait_for_element(element_id) -> bool:
  try:
    WebDriverWait(driver, timeout=10).until(EC.presence_of_element_located((By.ID, element_id)))
    time.sleep(5) # Some element might not be clickable immediately after it's loaded
    return True
  except TimeoutException:
    debug_print("Loading took too much time")
    return False
  except NoSuchElementException:
    debug_print("Element not found")
    return False

def intel_login():
  debug_print("Open login page")
  driver.get(LOGIN_URL)

  if not wait_for_element("signInName"):
    driver.get_screenshot_as_file("signInName.png")
    return False

  debug_print("Type username")
  action_chain.move_to_element(driver.find_element(By.ID, "signInName"))
  action_chain.click()
  action_chain.send_keys(USERNAME)
  action_chain.perform()

  debug_print("Click sign-in")
  action_chain.move_to_element(driver.find_element(By.ID, "continue"))
  action_chain.click()
  action_chain.perform()

  if wait_for_element("b2b_message_continue"):
    debug_print("Click Sign In")
    action_chain.move_to_element(driver.find_element(By.ID, "b2b_message_continue"))
    action_chain.click()
    action_chain.perform()

  if not wait_for_element(element_id="i0118"):
    driver.get_screenshot_as_file("password.png")
    return False

  debug_print("Type password")
  action_chain.move_to_element(driver.find_element(By.ID, "i0118"))
  action_chain.click()
  action_chain.send_keys(PASSWORD)
  action_chain.perform()

  debug_print("Click Sign In")
  action_chain.move_to_element(driver.find_element(By.ID, "idSIButton9"))
  action_chain.click()
  action_chain.perform()

  if not wait_for_element(element_id="idSIButton9"):
    driver.get_screenshot_as_file("click-yes.png")
    return False
  debug_print("Click Yes")
  action_chain.move_to_element(driver.find_element(By.ID, "idSIButton9"))
  action_chain.click()
  action_chain.perform()

  return True

def login():
  debug_print("Open login page")
  driver.get(LOGIN_URL)

  if not wait_for_element("signInName"):
    driver.get_screenshot_as_file("signInName.png")
    return False
  debug_print("Type username")
  action_chain.move_to_element(driver.find_element(By.ID, "signInName"))
  action_chain.click()
  action_chain.send_keys(USERNAME)
  action_chain.perform()

  debug_print("Click sign-in")
  action_chain.move_to_element(driver.find_element(By.ID, "continue"))
  action_chain.click()
  action_chain.perform()

  if not wait_for_element("password"):
    driver.get_screenshot_as_file("password.png")
    return False
  debug_print("Type password")
  action_chain.move_to_element(driver.find_element(By.ID, "password"))
  action_chain.click()
  action_chain.send_keys(PASSWORD)
  action_chain.perform()

  debug_print("Click Sign In")
  action_chain.move_to_element(driver.find_element(By.ID, "continue"))
  action_chain.click()
  action_chain.perform()

  return True

def b2b_login():
  debug_print("Open login page")
  driver.get(LOGIN_URL)

  if not wait_for_element("i0116"):
    driver.get_screenshot_as_file("i0116.png")
    return False
  debug_print("Type username")
  action_chain.move_to_element(driver.find_element(By.ID, "i0116"))
  action_chain.click()
  action_chain.send_keys(USERNAME)
  action_chain.perform()

  debug_print("Click sign-in")
  action_chain.move_to_element(driver.find_element(By.ID, "idSIButton9"))
  action_chain.click()
  action_chain.perform()

  if not wait_for_element(element_id="i0118"):
    driver.get_screenshot_as_file("i0118.png")
    return False

  debug_print("Type password")
  action_chain.move_to_element(driver.find_element(By.ID, "i0118"))
  action_chain.click()
  action_chain.send_keys(PASSWORD)
  action_chain.perform()

  debug_print("Click Sign In")
  action_chain.move_to_element(driver.find_element(By.ID, "idSIButton9"))
  action_chain.click()
  action_chain.perform()

  if not wait_for_element(element_id="idSIButton9"):
    driver.get_screenshot_as_file("click-yes.png")
    # If the element is not found, it might be because we already get redirected to token page.
    return True

  if wait_for_element("refreshToken"):
    # Additional check to ensure we arrived at the token page
    return True

  debug_print("Click Yes")
  action_chain.move_to_element(driver.find_element(By.ID, "idSIButton9"))
  action_chain.click()
  action_chain.perform()

  return True

def get_refresh_token():
  if not wait_for_element("refreshToken"):
    driver.get_screenshot_as_file("refreshToken.png")
  elem = driver.find_element(By.ID, "refreshToken")
  return elem.get_attribute("value")

try:
  result = False
  # Case 1: Login to B2C with Intel account
  if azure_ad_b2c and USERNAME.endswith("@intel.com"):
    result = intel_login()

  # Case 2: Login to B2C with non-Intel account
  elif azure_ad_b2c:
    result = login()

  # Case 3: Login to B2B with Intel Account
  else:
    result = b2b_login()
  if result:
    refresh_token = get_refresh_token()
    print(refresh_token)
except Exception as e:
  print(e)
finally:
  driver.quit()
