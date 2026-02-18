# **NS116 Brand Identity & UI Style Guide**

## **1. The Concept**

**NS116** is an open-source web interface for AWS Route53. Its name and identity
are a direct homage to the physical infrastructure that connects us.

The brand moves away from abstract cloud metaphors and embraces the literal
**highway shield** symbolism. It represents the "Route" in Route53, drawing a
parallel between the **BR-116** (Brazil's longest highway) and the **DNS
protocol** (the internet's navigation system). The aesthetic is functional,
infrastructure-inspired, and highly legible.

## **2. Visual Identity**

### **2.1 The Logo**

The primary mark is a monochromatic, badge-style highway shield, divided
horizontally.

* **The Shape:** A classic federal highway shield with a flat top and pointed
  bottom.
* **Top Compartment:** Contains the text **"DNS"** (The Protocol).
* **Bottom Compartment:** Contains the text **"NS 116"** (The Route/Project
  Name).
* **The Border:** A double-stroke outline (thick inner, thin outer) ensuring
  visibility on any background.

### **2.2 Typography**

* **Headings & Display:** Highway Gothic (Roadgeek 2005) or Overpass
  * *Why:* These fonts mimic the official typeface used on road signage
      worldwide. They provide the authentic "infrastructure" look.
* **Body Text:** Inter or Helvetica Now
  * *Why:* Neutral, neo-grotesque sans-serifs that maintain the clean,
      Swiss-style aesthetic of the shield.
* **Code & Technical Data:** JetBrains Mono or Fira Code
  * *Why:* Monospaced legibility is crucial for IP addresses and JSON records.

### **2.3 Color Palette**

The brand utilizes a **Functional Chromatic** system inspired by road signage.

| Color | Hex Code | Purpose |
| :---- | :---- | :---- |
| **Highway Green** | `#009B3A` | Primary actions, success states, background accents. Represents "Go" and safe routing. |
| **Caution Yellow** | `#FFCC00` | Warnings, editing states, attention-grabbing interactive elements. |
| **Connection Blue** | `#007BFF` | Links, information, active states, focus rings. |
| **Asphalt Dark** | `#1A1A1A` | Primary text, headers, dark backgrounds. |
| **Reflective White** | `#F8F9FA` | Main background, clean surfaces. |
| **Road Gray** | `#E9ECEF` | Borders, dividers, disabled controls. |

## **3. Usage Guidelines**

### **3.1 Correct Usage (The "Dos")**

* **Aspect Ratio:** The shield represents a physical sign. Never stretch or skew
  the dimensions.
* **Positive/Negative:**
  * **Light Mode:** Dark text on Light background.
  * **Dark Mode:** Light text on Dark background.
* **Contextual Color:** Use Green for "Safe/Active", Yellow for "Caution/Change",
  Red for "Danger/Stop".

### **3.2 Incorrect Usage (The "Don'ts")**

* **No Random Colors:** Stick to the defined palette.
* **No Unnecessary Gradients:** Use flat colors or very subtle gradients for
  depth.
* **No "Ghosting":** Do not reduce opacity of primary text.

## **4. UI & Interface Patterns**

The UI follows a **"Highway / Infrastructure"** aesthetic—modern, clean, and highly
functional, with subtle animations to provide feedback.

### **4.1 Layout Principles**

* **Card-Based Layout:** Content is organized in white cards with soft shadows
  and subtle border accents.
* **Hover Effects:** Cards and interactive elements react to mouse hover
  (elevation lift, border color change) to invite interaction.
* **Signage Typography:** Clear, bold headings using **Overpass** or **Highway
  Gothic** styles.

### **4.2 Buttons & Actions**

* **Primary Action (Add/Create):** Highway Green background, White text, rounded
  corners (8px), slight shadow.
* **Secondary Action (Refresh/Cancel):** White background, Gray border, Dark
  text.
* **Interactive Feedback:** Buttons often include icons (using Lucide) and subtle
  transformations (scale, color shift) on click/hover.
  * *Example:* The Refresh button spins its icon when clicked.

### **4.3 Visual Hierarchy**

1. **Zone/Page Title:** H1/H2, Overpass Bold, Asphalt Dark.
2. **Badges (Record Type):** Color-coded pills (Green for A, Purple for CNAME,
    etc.) for quick visual scanning.
3. **Data Data:** Monospace font (Fira Code) for IP addresses, DNS values, and
    TTLs.

## **5. Website / Repository Assets**

When hosting the project on GitHub or a landing page:

* **Favicon:** The full shield logo (it scales well due to high contrast).
* **Social Preview / OG Image:** The Shield logo centered on a white background,
  with "NS116" typed in Highway Gothic below it.
* **Readme Header:** A minimal, high-contrast banner. White background with the
  Shield logo aligned left and the project title in large, black typography.
