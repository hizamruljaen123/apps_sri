function fetchDataAndDisplay() {
    fetch('http://localhost:8080/get_data')
      .then(response => response.json())
      .then(data => {
        const tableBody = document.querySelector('#data-table tbody');
        tableBody.innerHTML = '';  // Clear any existing rows
  
        data.forEach(item => {
          const row = `
            <tr>
              <td class="border px-4 py-2 text-gray-700">${item.gejala}</td>
              <td class="border px-4 py-2 text-gray-700">${item.bobot_gejala}</td>
              <td class="border px-4 py-2 text-gray-700">${item.importance}</td>
              <td class="border px-4 py-2 text-gray-700">${item.nama_penyakit}</td>
            </tr>
          `;
          tableBody.insertAdjacentHTML('beforeend', row);
        });
      })
      .catch(error => {
        console.error('Error fetching data:', error);
      });
  }
  

  
  // Call the function when needed
  fetchDataAndDisplay();
  